package stdocs

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"sort"
	"strings"
	"sync"

	"github.com/FumingPower3925/stdocs/internal/schema"
)

// DriftWarn wraps the mux in a development aid that compares what
// handlers actually do against what the document claims, and logs a
// warning on divergence:
//
//   - a handler writes a status code its operation does not document
//     (and no "default" response is declared), or
//   - a response documented with a JSON body is written with a
//     non-JSON Content-Type.
//
// Each (route, finding) pair warns once, so a hot endpoint does not
// flood the log. logf receives Printf-style arguments; nil means
// [log.Printf].
//
//	handler := mux // production
//	if os.Getenv("ENV") == "dev" {
//	    handler = stdocs.DriftWarn(mux, nil)
//	}
//	log.Fatal(http.ListenAndServe(":8080", handler))
//
// DriftWarn builds the document up front, so the fail-fast panics
// from invalid constraint or params tags surface at the DriftWarn
// call; routes registered later are picked up automatically on the
// next request. A mux-level "default" response counts as documenting
// any status — but its body contract still applies: a JSON-documented
// default served with a non-JSON Content-Type is drift.
//
// DriftWarn is a development aid, not validation: it checks
// responses only, by design — request bodies and parameters are
// never sampled — and adds a small per-request bookkeeping cost, so
// wrap only in environments where the warnings are read. Routes
// excluded from the document (Hidden, non-shown Internal) and
// requests under the docs prefix are ignored. The documented-status
// set is the served document's: mux-level default responses and the
// automatic 401 count as documented. [DriftNotify] delivers the same
// findings in structured form for CI gates and dashboards.
func DriftWarn(m *Mux, logf func(format string, args ...any), opts ...DriftOption) http.Handler {
	if logf == nil {
		logf = log.Printf
	}
	d := &driftWarner{mux: m, logf: logf, seen: make(map[string]bool)}
	for _, o := range opts {
		if o != nil {
			o(d)
		}
	}
	d.refresh()
	return d
}

// DriftOption configures DriftWarn.
type DriftOption func(*driftWarner)

// DriftSampleBodies additionally compares response bodies against the
// documented schema, for responses documented with a JSON object or
// array-of-objects schema: each missing documented-required key warns
// once per route, status, and field; the appearance of undocumented
// keys warns once per route and status. The comparison covers
// top-level keys and one level of rows — elements of array-of-object
// properties ("orders[].fee_cents") and of array bodies ("[].id") —
// never values, deeper nesting, or rows of rows. Sampling copies up
// to 64 KB of each tracked response, which is why it is a separate
// opt; like DriftWarn itself, it is a development aid.
func DriftSampleBodies() DriftOption {
	return func(d *driftWarner) { d.sampleBodies = true }
}

// DriftFinding is one drift observation in structured form, delivered
// to [DriftNotify] callbacks alongside the log line.
type DriftFinding struct {
	// Code identifies the finding kind stably across releases —
	// allow-lists and CI gates should match on it, never on Message
	// prose, which is free to improve. The same discipline as
	// [Warning].Code. The codes:
	//
	//	build-failed           the document could not be built;
	//	                       findings may be incomplete
	//	undeclared-status      a handler wrote a status the document
	//	                       does not declare
	//	content-type-mismatch  the served Content-Type contradicts
	//	                       the declared body
	//	missing-required-field a sampled body lacks a key its schema
	//	                       requires (DriftSampleBodies)
	//	undocumented-fields    a sampled body carries keys its schema
	//	                       does not document (DriftSampleBodies)
	Code string
	// Route is the registered pattern ("GET /tasks"); empty for
	// document-level findings (build-failed).
	Route string
	// Status is the observed response status; 0 for document-level
	// findings.
	Status int
	// Field locates body findings: a top-level key ("fee_cents"), a
	// row path ("orders[].fee_cents", "[].id"), or the row prefix
	// alone when rows carry undocumented fields ("orders[]"). Empty
	// for everything else.
	Field string
	// Message is the human-readable line — the logged text without
	// its "stdocs drift: " prefix.
	Message string
}

// DriftNotify registers fn to receive every finding DriftWarn would
// log, in structured form — the bridge from log lines to CI gates
// and dashboards. fn runs once per deduplicated finding, on the
// goroutine serving the request that observed it, so it must
// synchronize anything it touches:
//
//	var mu sync.Mutex
//	var found []stdocs.DriftFinding
//	h := stdocs.DriftWarn(mux, nil, stdocs.DriftNotify(func(f stdocs.DriftFinding) {
//	    mu.Lock()
//	    defer mu.Unlock()
//	    found = append(found, f)
//	}))
//
// Replaying traffic through h in a test and asserting on the
// collected findings turns drift into a CI gate, with the same
// allow-list shape as [Mux.Lint]:
//
//	accepted := map[string]bool{"undocumented-fields": true}
//	for _, f := range found {
//	    if !accepted[f.Code] {
//	        t.Errorf("%s", f.Message)
//	    }
//	}
//
// A gate is only as strong as its traffic — findings exist for
// exercised routes only — and build-failed arrives through fn too,
// so a broken document cannot pass as zero drift. Findings are
// deduplicated for the warner's lifetime; multiple DriftNotify
// callbacks all receive each finding.
func DriftNotify(fn func(DriftFinding)) DriftOption {
	return func(d *driftWarner) {
		if fn != nil {
			d.notify = append(d.notify, fn)
		}
	}
}

// driftShape is the documented top-level object shape for one
// response entry, plus the row shapes one level down: array-of-object
// properties carry their element shape in rows, and a body that is
// itself an array of objects carries it in arrayRow.
type driftShape struct {
	props    map[string]bool
	required []string
	rows     map[string]driftShape
	arrayRow *driftShape
}

// driftRoute is the immutable per-route snapshot the request path
// checks against — copied out of the route's finalized operation
// under the build lock, so serving never touches state that
// finalize/Refresh mutate.
type driftRoute struct {
	statuses    map[string]bool // documented response keys
	hasDefault  bool
	jsonByKey   map[string]bool   // key -> declared with a JSON body
	defaultJSON bool              // the "default" entry declares a JSON body
	ctByKey     map[string]string // key -> declared non-JSON media type (base)
	shapeByKey  map[string]driftShape
}

// driftWarner is the http.Handler returned by DriftWarn.
type driftWarner struct {
	mux          *Mux
	logf         func(format string, args ...any)
	sampleBodies bool
	notify       []func(DriftFinding)

	mu      sync.Mutex
	seen    map[string]bool
	snapGen uint64
	routes  map[string]driftRoute
}

// refresh rebuilds the route snapshot and reports any build error —
// outside the build lock, so notify callbacks are free to call the
// mux's document methods.
func (d *driftWarner) refresh() {
	if err := d.rebuild(); err != nil {
		d.emit("\x00build-failed\x00"+err.Error(), DriftFinding{Code: "build-failed"},
			"document build failed, warnings may be incomplete: %v", err)
	}
}

// rebuild copies the route snapshot under the build lock, so finalize
// cannot mutate operations mid-snapshot; the copied maps are never
// shared.
func (d *driftWarner) rebuild() error {
	d.mux.specMu.Lock()
	defer d.mux.specMu.Unlock()
	// The generation is captured before building — the same
	// lost-update discipline as cachedJSON: a route registered while
	// we build bumps the generation past what we record, so the next
	// request refreshes again instead of trusting a snapshot that may
	// hold the late route unfinalized.
	_, gen := d.mux.reg.snapshot()
	// Building finalizes the visible routes' operations (auto-200,
	// default responses, auto-401), so the snapshot reflects exactly
	// what the document says. A build error (an unregistered security
	// scheme) is the document's problem — note it once and snapshot
	// what we can.
	_, buildErr := d.mux.cachedJSON()
	routes := make(map[string]driftRoute, 16)
	for _, rt := range d.mux.visibleRoutes() {
		dr := driftRoute{
			statuses:  make(map[string]bool, len(rt.op.Responses)),
			jsonByKey: make(map[string]bool, len(rt.op.Responses)),
			ctByKey:   make(map[string]string, len(rt.op.Responses)),
		}
		for key, resp := range rt.op.Responses {
			dr.statuses[key] = true
			declaredJSON := resp != nil && (resp.BodyValue != nil || resp.Schema != nil) &&
				(resp.ContentType == "" || strings.Contains(resp.ContentType, "json"))
			dr.jsonByKey[key] = declaredJSON
			if resp != nil && resp.ContentType != "" && !strings.Contains(resp.ContentType, "json") {
				dr.ctByKey[key] = mediaTypeBase(resp.ContentType)
			}
			if key == "default" {
				dr.hasDefault = true
				dr.defaultJSON = declaredJSON
			}
			if d.sampleBodies && declaredJSON && resp.BodyValue != nil {
				if shape, ok := objectShape(resp.BodyValue); ok {
					if dr.shapeByKey == nil {
						dr.shapeByKey = make(map[string]driftShape)
					}
					dr.shapeByKey[key] = shape
				}
			}
		}
		routes[rt.pattern] = dr
	}
	d.mu.Lock()
	d.routes = routes
	d.snapGen = gen
	d.mu.Unlock()
	return buildErr
}

// snapshot returns the current route map, rebuilding it when routes
// were registered since the last build.
func (d *driftWarner) snapshot() map[string]driftRoute {
	_, gen := d.mux.reg.snapshot()
	d.mu.Lock()
	stale := gen != d.snapGen
	routes := d.routes
	d.mu.Unlock()
	if stale {
		d.refresh()
		d.mu.Lock()
		routes = d.routes
		d.mu.Unlock()
	}
	return routes
}

func (d *driftWarner) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	_, pattern := d.mux.Handler(r)
	dr, tracked := d.snapshot()[pattern]
	if !tracked {
		// Unregistered handlers, the 404 fallback, docs-prefix routes,
		// and routes excluded from the document have no contract to
		// drift from.
		d.mux.ServeHTTP(w, r)
		return
	}
	rec := &driftRecorder{ResponseWriter: w, captureBody: d.sampleBodies && len(dr.shapeByKey) > 0}
	d.mux.ServeHTTP(rec, r)

	if rec.hijacked {
		// The handler took over the connection (websocket upgrade);
		// there is no HTTP response to compare.
		return
	}
	status := rec.status
	if status == 0 {
		status = http.StatusOK // a body written without WriteHeader
	}
	key := itoa(status)
	declaredJSON, declared := false, false
	switch {
	case dr.statuses[key]:
		declared = true
		declaredJSON = dr.jsonByKey[key]
	case dr.hasDefault:
		// The default entry covers the status — including its body
		// contract: a JSON-documented default served as text/plain is
		// drift even though the status itself is covered.
		declared = true
		declaredJSON = dr.defaultJSON
	}
	if !declared {
		// Method-less patterns match any verb, so the observed method
		// is real information there; on "POST /x" registrations it
		// would just repeat the pattern.
		via := ""
		if !strings.Contains(pattern, " ") {
			via = " (observed via " + r.Method + ")"
		}
		d.emit(pattern+"\x00"+key, DriftFinding{Code: "undeclared-status", Route: pattern, Status: status},
			"%s%s returned %d, which the document does not declare",
			pattern, via, status)
		return
	}
	if declaredJSON {
		ct := rec.Header().Get("Content-Type")
		if ct != "" && !strings.Contains(ct, "json") {
			d.emit(pattern+"\x00"+key+"\x00ct", DriftFinding{Code: "content-type-mismatch", Route: pattern, Status: status},
				"%s wrote Content-Type %q for status %d, which the document declares with a JSON body", pattern, ct, status)
			return
		}
		d.checkBodyShape(dr, pattern, key, status, rec)
		return
	}
	// Declared non-JSON contracts are contracts too: a route
	// documented as text/csv serving text/plain (or JSON) is drift —
	// including when the status is covered by the default entry,
	// whose declared media type is then the contract.
	ctKey := key
	if !dr.statuses[key] {
		ctKey = "default"
	}
	if declared := dr.ctByKey[ctKey]; declared != "" {
		if served := mediaTypeBase(rec.Header().Get("Content-Type")); served != "" && served != declared {
			d.emit(pattern+"\x00"+key+"\x00ct", DriftFinding{Code: "content-type-mismatch", Route: pattern, Status: status},
				"%s wrote Content-Type %q for status %d, which the document declares as %s", pattern, served, status, declared)
		}
	}
}

// mediaTypeBase strips parameters from a media type ("text/csv;
// charset=utf-8" -> "text/csv").
func mediaTypeBase(ct string) string {
	if i := strings.IndexByte(ct, ';'); i >= 0 {
		ct = ct[:i]
	}
	return strings.TrimSpace(strings.ToLower(ct))
}

// checkBodyShape compares a sampled response body's top-level keys
// against the documented object shape, when sampling is on and a
// shape was recorded for the status (or the default entry).
func (d *driftWarner) checkBodyShape(dr driftRoute, pattern, key string, status int, rec *driftRecorder) {
	if !d.sampleBodies || rec.bodyOverflow {
		return
	}
	shape, ok := dr.shapeByKey[key]
	if !ok {
		if dr.statuses[key] {
			// Declared specifically but without an object shape
			// (body-less, raw, or non-object): nothing to compare.
			return
		}
		// The status falls under the default entry — compare against
		// its shape when it has one.
		if shape, ok = dr.shapeByKey["default"]; !ok {
			return
		}
	}
	if shape.arrayRow != nil {
		var rows []any
		if json.Unmarshal(rec.body.Bytes(), &rows) != nil {
			return // not a JSON array; the schema check has nothing to say
		}
		d.checkRows(pattern, key, status, "[]", *shape.arrayRow, rows)
		return
	}
	var body map[string]any
	if json.Unmarshal(rec.body.Bytes(), &body) != nil {
		return // not a JSON object; the schema check has nothing to say
	}
	if body == nil {
		return // a literal null declares nothing about fields
	}
	for _, req := range shape.required {
		if _, present := body[req]; !present {
			d.emit(pattern+"\x00"+key+"\x00breq\x00"+req, DriftFinding{Code: "missing-required-field", Route: pattern, Status: status, Field: req},
				"%s response %d is missing required field %q declared by its documented schema", pattern, status, req)
		}
	}
	var extras []string
	for k := range body {
		if !shape.props[k] {
			extras = append(extras, k)
		}
	}
	if len(extras) > 0 {
		sort.Strings(extras)
		if len(extras) > 3 {
			extras = extras[:3]
		}
		d.emit(pattern+"\x00"+key+"\x00bextra", DriftFinding{Code: "undocumented-fields", Route: pattern, Status: status},
			"%s response %d carries fields not in its documented schema (e.g. %s)", pattern, status, strings.Join(extras, ", "))
	}
	for prop, row := range shape.rows {
		if items, ok := body[prop].([]any); ok {
			d.checkRows(pattern, key, status, prop+"[]", row, items)
		}
	}
}

// checkRows compares sampled rows' keys against the documented row
// shape, accumulating across rows first so warn volume is bounded by
// the schema, never the row count. Null and non-object entries
// declare nothing about fields and are skipped.
func (d *driftWarner) checkRows(pattern, key string, status int, path string, row driftShape, items []any) {
	missing := make(map[string]bool)
	extraSet := make(map[string]bool)
	sampled := false
	for _, item := range items {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		sampled = true
		for _, req := range row.required {
			if _, present := m[req]; !present {
				missing[req] = true
			}
		}
		for k := range m {
			if !row.props[k] {
				extraSet[k] = true
			}
		}
	}
	if !sampled {
		return
	}
	for _, req := range row.required {
		if missing[req] {
			qualified := path + "." + req
			d.emit(pattern+"\x00"+key+"\x00breq\x00"+qualified, DriftFinding{Code: "missing-required-field", Route: pattern, Status: status, Field: qualified},
				"%s response %d is missing required field %q declared by its documented schema", pattern, status, qualified)
		}
	}
	if len(extraSet) > 0 {
		extras := make([]string, 0, len(extraSet))
		for k := range extraSet {
			extras = append(extras, path+"."+k)
		}
		sort.Strings(extras)
		if len(extras) > 3 {
			extras = extras[:3]
		}
		d.emit(pattern+"\x00"+key+"\x00bextra\x00"+path, DriftFinding{Code: "undocumented-fields", Route: pattern, Status: status, Field: path},
			"%s response %d carries fields not in its documented schema (e.g. %s)", pattern, status, strings.Join(extras, ", "))
	}
}

// objectShape reflects a response body value and resolves it to its
// top-level shape, when it has one: an object's keys and one level of
// array-of-object row shapes, or the row shape alone when the body is
// itself an array.
func objectShape(bodyValue any) (driftShape, bool) {
	root, comps := schema.ReflectSchema(bodyValue)
	s := resolveShapeRef(root, comps)
	if s == nil {
		return driftShape{}, false
	}
	if s.Type == "array" {
		if row := rowShape(s.Items, comps); row != nil {
			return driftShape{arrayRow: row}, true
		}
		return driftShape{}, false
	}
	if s.Type != "object" || len(s.Properties) == 0 {
		return driftShape{}, false
	}
	shape := driftShape{props: make(map[string]bool, len(s.Properties))}
	for name, p := range s.Properties {
		shape.props[name] = true
		if p = resolveShapeRef(p, comps); p != nil && p.Type == "array" {
			if row := rowShape(p.Items, comps); row != nil {
				if shape.rows == nil {
					shape.rows = make(map[string]driftShape)
				}
				shape.rows[name] = *row
			}
		}
	}
	shape.required = append(shape.required, s.Required...)
	sort.Strings(shape.required)
	return shape, true
}

// rowShape resolves an array's element schema to an object shape —
// one level only; rows of rows are out of sampling's scope.
func rowShape(items *schema.Schema, comps map[string]*schema.Schema) *driftShape {
	s := resolveShapeRef(items, comps)
	if s == nil || s.Type != "object" || len(s.Properties) == 0 {
		return nil
	}
	row := driftShape{props: make(map[string]bool, len(s.Properties))}
	for name := range s.Properties {
		row.props[name] = true
	}
	row.required = append(row.required, s.Required...)
	sort.Strings(row.required)
	return &row
}

// resolveShapeRef follows a $ref into the reflected components.
func resolveShapeRef(s *schema.Schema, comps map[string]*schema.Schema) *schema.Schema {
	if s != nil && s.Ref != "" {
		name := strings.TrimPrefix(s.Ref, "#/components/schemas/")
		return comps[name]
	}
	return s
}

// emit logs once per dedup key and delivers the structured finding to
// the notify callbacks. Formatting is deferred past the dedup check,
// and callbacks run outside every lock.
func (d *driftWarner) emit(dedup string, f DriftFinding, format string, args ...any) {
	d.mu.Lock()
	already := d.seen[dedup]
	d.seen[dedup] = true
	d.mu.Unlock()
	if already {
		return
	}
	f.Message = fmt.Sprintf(format, args...)
	d.logf("stdocs drift: %s", f.Message)
	for _, fn := range d.notify {
		fn(f)
	}
}

// driftRecorder captures the response status while passing everything
// through. It preserves the optional ResponseWriter interfaces:
// Unwrap serves http.ResponseController, and Flush/Hijack/ReadFrom
// forward so streaming (SSE), websocket upgrades, and sendfile keep
// working through the wrapper.
type driftRecorder struct {
	http.ResponseWriter
	status   int
	hijacked bool

	captureBody  bool
	body         bytes.Buffer
	bodyOverflow bool
}

// driftBodyCap bounds how much response body sampling will buffer.
const driftBodyCap = 64 << 10

func (r *driftRecorder) WriteHeader(status int) {
	// 1xx informational responses (e.g. 103 Early Hints) are not the
	// final status; net/http lets the handler call WriteHeader again.
	if r.status == 0 && (status >= 200 || status == http.StatusSwitchingProtocols) {
		r.status = status
	}
	r.ResponseWriter.WriteHeader(status)
}

func (r *driftRecorder) Write(b []byte) (int, error) {
	if r.status == 0 {
		r.status = http.StatusOK
	}
	if r.captureBody && !r.bodyOverflow {
		if r.body.Len()+len(b) > driftBodyCap {
			r.bodyOverflow = true
			r.body.Reset()
		} else {
			r.body.Write(b)
		}
	}
	return r.ResponseWriter.Write(b)
}

// Unwrap lets http.ResponseController reach the underlying writer.
func (r *driftRecorder) Unwrap() http.ResponseWriter { return r.ResponseWriter }

// Flush implements http.Flusher for handlers that assert it directly.
func (r *driftRecorder) Flush() {
	if r.status == 0 {
		r.status = http.StatusOK
	}
	// A flush failure has nowhere to go in a dev aid; the handler's
	// own writes will surface the broken connection.
	_ = http.NewResponseController(r.ResponseWriter).Flush()
}

// Hijack implements http.Hijacker for handlers that assert it
// directly (websocket libraries).
func (r *driftRecorder) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	conn, rw, err := http.NewResponseController(r.ResponseWriter).Hijack()
	if err == nil {
		r.hijacked = true
	}
	return conn, rw, err
}

// ReadFrom preserves the sendfile fast path when the underlying
// writer supports it — except while sampling, where the bytes must
// pass through Write to reach the capture buffer.
func (r *driftRecorder) ReadFrom(src io.Reader) (int64, error) {
	if r.status == 0 {
		r.status = http.StatusOK
	}
	if r.captureBody {
		return io.Copy(struct{ io.Writer }{r}, src)
	}
	if rf, ok := r.ResponseWriter.(io.ReaderFrom); ok {
		return rf.ReadFrom(src)
	}
	return io.Copy(struct{ io.Writer }{r.ResponseWriter}, src)
}
