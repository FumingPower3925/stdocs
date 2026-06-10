package stdocs

import (
	"log"
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
	// Building the document finalizes the visible routes' operations
	// (auto-200, default responses, auto-401), so the runtime check
	// compares against exactly what the document says. A build error
	// (e.g. an unregistered security scheme) is the document's
	// problem, not this tool's — report it once and check what we
	// can.
	if _, err := m.JSON(); err != nil {
		logf("stdocs drift: document build failed, warnings may be incomplete: %v", err)
	}
	routes := make(map[string]*route, len(m.reg.routes))
	for _, rt := range m.visibleRoutes() {
		routes[rt.pattern] = rt
	}
	d := &driftWarner{mux: m, logf: logf, routes: routes, seen: make(map[string]bool)}
	return d
}

// driftWarner is the http.Handler returned by DriftWarn.
type driftWarner struct {
	mux    *Mux
	logf   func(format string, args ...any)
	routes map[string]*route

	mu   sync.Mutex
	seen map[string]bool
}

func (d *driftWarner) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	_, pattern := d.mux.Handler(r)
	rt, tracked := d.routes[pattern]
	if !tracked {
		// Unregistered handlers, the 404 fallback, docs-prefix routes,
		// and routes excluded from the document have no contract to
		// drift from.
		d.mux.ServeHTTP(w, r)
		return
	}
	rec := &driftRecorder{ResponseWriter: w}
	d.mux.ServeHTTP(rec, r)

	status := rec.status
	if status == 0 {
		status = http.StatusOK // a body written without WriteHeader
	}
	key := itoa(status)
	resp, documented := rt.op.Responses[key]
	if !documented {
		if _, hasDefault := rt.op.Responses["default"]; !hasDefault {
			d.warn(pattern+"\x00"+key,
				"stdocs drift: %s returned %d, which the document does not declare", pattern, status)
		}
		return
	}
	if resp != nil && (resp.BodyValue != nil || resp.Schema != nil) {
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
// through.
type driftRecorder struct {
	http.ResponseWriter
	status int
}

func (r *driftRecorder) WriteHeader(status int) {
	if r.status == 0 {
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
