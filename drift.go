package stdocs

import (
	"bufio"
	"io"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
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
// DriftWarn is a development aid, not validation: it observes
// statuses and content types, never request or body schemas, and adds
// a small per-request bookkeeping cost — wrap only in environments
// where the warnings are read. Routes excluded from the document
// (Hidden, non-shown Internal) and requests under the docs prefix are
// ignored. The documented-status set is the served document's:
// mux-level default responses and the automatic 401 count as
// documented.
func DriftWarn(m *Mux, logf func(format string, args ...any)) http.Handler {
	if logf == nil {
		logf = log.Printf
	}
	d := &driftWarner{mux: m, logf: logf, seen: make(map[string]bool)}
	d.refresh()
	return d
}

// driftRoute is the immutable per-route snapshot the request path
// checks against — copied out of the route's finalized operation
// under the build lock, so serving never touches state that
// finalize/Refresh mutate.
type driftRoute struct {
	statuses    map[string]bool // documented response keys
	hasDefault  bool
	jsonByKey   map[string]bool // key -> declared with a JSON body
	defaultJSON bool            // the "default" entry declares a JSON body
}

// driftWarner is the http.Handler returned by DriftWarn.
type driftWarner struct {
	mux  *Mux
	logf func(format string, args ...any)

	mu      sync.Mutex
	seen    map[string]bool
	snapGen uint64
	routes  map[string]driftRoute
}

// refresh rebuilds the route snapshot from the current document. The
// build lock is held for the whole copy, so finalize cannot mutate
// operations mid-snapshot; the copied maps are never shared.
func (d *driftWarner) refresh() {
	d.mux.specMu.Lock()
	defer d.mux.specMu.Unlock()
	// Building finalizes the visible routes' operations (auto-200,
	// default responses, auto-401), so the snapshot reflects exactly
	// what the document says. A build error (an unregistered security
	// scheme) is the document's problem — note it once and snapshot
	// what we can.
	if _, err := d.mux.cachedJSON(); err != nil {
		d.logf("stdocs drift: document build failed, warnings may be incomplete: %v", err)
	}
	routes := make(map[string]driftRoute, len(d.mux.reg.routes))
	for _, rt := range d.mux.visibleRoutes() {
		dr := driftRoute{
			statuses:  make(map[string]bool, len(rt.op.Responses)),
			jsonByKey: make(map[string]bool, len(rt.op.Responses)),
		}
		for key, resp := range rt.op.Responses {
			dr.statuses[key] = true
			declaredJSON := resp != nil && (resp.BodyValue != nil || resp.Schema != nil) &&
				(resp.ContentType == "" || strings.Contains(resp.ContentType, "json"))
			dr.jsonByKey[key] = declaredJSON
			if key == "default" {
				dr.hasDefault = true
				dr.defaultJSON = declaredJSON
			}
		}
		routes[rt.pattern] = dr
	}
	d.routes = routes
	d.snapGen = d.mux.reg.gen
}

// snapshot returns the current route map, rebuilding it when routes
// were registered since the last build.
func (d *driftWarner) snapshot() map[string]driftRoute {
	d.mux.specMu.Lock()
	gen := d.mux.reg.gen
	d.mux.specMu.Unlock()
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
	rec := &driftRecorder{ResponseWriter: w}
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
		d.warn(pattern+"\x00"+key,
			"stdocs drift: %s (observed via %s) returned %d, which the document does not declare",
			pattern, r.Method, status)
		return
	}
	if declaredJSON {
		ct := rec.Header().Get("Content-Type")
		if ct != "" && !strings.Contains(ct, "json") {
			d.warn(pattern+"\x00"+key+"\x00ct",
				"stdocs drift: %s wrote Content-Type %q for status %d, which the document declares with a JSON body", pattern, ct, status)
		}
	}
}

// warn logs once per key.
func (d *driftWarner) warn(key, format string, args ...any) {
	d.mu.Lock()
	already := d.seen[key]
	d.seen[key] = true
	d.mu.Unlock()
	if !already {
		d.logf(format, args...)
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
}

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
// writer supports it.
func (r *driftRecorder) ReadFrom(src io.Reader) (int64, error) {
	if r.status == 0 {
		r.status = http.StatusOK
	}
	if rf, ok := r.ResponseWriter.(io.ReaderFrom); ok {
		return rf.ReadFrom(src)
	}
	return io.Copy(struct{ io.Writer }{r.ResponseWriter}, src)
}
