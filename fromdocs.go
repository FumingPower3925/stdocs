package stdocs

import (
	"net/http"
	"net/url"
	"strings"
)

// FromDocs reports whether r appears to originate from the interactive
// docs UI served under docsPrefix — that is, from a "Try it out" /
// "Test Request" console on the docs page rather than from a regular
// API client. An empty (or all-slash) docsPrefix means the default
// "/docs".
//
// Detection is based on the Referer header: browsers attach the docs
// page's URL to the fetch calls the consoles make. The check matches
// on the URL path only, so it keeps working behind reverse proxies
// that add their own path prefix (a page at /api/docs/ still matches
// a docs prefix of "/docs").
//
// FromDocs is a convenience guardrail, NOT a security control. The
// Referer header is fully client-controlled: a caller can forge it
// (false positives), and there are false negatives too — privacy
// extensions or a strict Referrer-Policy can strip the header, and
// try-it requests sent to a DIFFERENT origin (an absolute WithServer
// URL on another host) carry an origin-only Referer under browsers'
// default policy, so FromDocs reports false for them (Scalar
// additionally routes cross-origin try-it calls through its own
// proxy). Detection is reliable only when the docs page and the API
// share an origin — the normal stdocs setup, where one mux serves
// both. Use FromDocs only to RESTRICT what docs-originated traffic
// may do — never to grant access, skip authentication, or relax
// validation. Endpoints that must not be mutated by strangers need
// real authentication regardless.
//
// The intended use is a small middleware in front of the mux:
//
//	guard := func(next http.Handler) http.Handler {
//	    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
//	        if r.Method != http.MethodGet && stdocs.FromDocs(r, "/docs") {
//	            http.Error(w, "try-it requests cannot modify data", http.StatusForbidden)
//	            return
//	        }
//	        next.ServeHTTP(w, r)
//	    })
//	}
//	log.Fatal(http.ListenAndServe(":8080", guard(mux)))
//
// Teams that prefer other policies can branch on FromDocs however
// they like: route writes to a scratch datastore, add a dry-run flag,
// tag the request for observability, and so on.
func FromDocs(r *http.Request, docsPrefix string) bool {
	referer := r.Referer()
	if referer == "" {
		return false
	}
	u, err := url.Parse(referer)
	if err != nil {
		return false
	}
	prefix := "/docs"
	if trimmed := strings.Trim(docsPrefix, "/"); trimmed != "" {
		prefix = "/" + trimmed
	}
	// The docs page lives at <prefix>/ (possibly below a proxy's own
	// path prefix), so match the path segment anywhere in the referring
	// URL's path. The leading "/" in prefix makes the match
	// boundary-safe ("/mydocs/" does not match prefix "/docs"), and
	// deliberately loose matching errs toward true (as does matching
	// on the percent-decoded path): FromDocs gates restrictions, so a
	// false positive is the safe direction.
	return u.Path == prefix || strings.Contains(u.Path, prefix+"/")
}

// FromDocs reports whether r appears to originate from this mux's
// docs UI. It is FromDocs(r, prefix) with the mux's configured docs
// prefix; see the package-level [FromDocs] for the detection
// mechanics and the security caveats.
func (m *Mux) FromDocs(r *http.Request) bool {
	return FromDocs(r, m.cfg.DocsPrefix)
}
