package stdocs

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func reqWithReferer(referer string) *http.Request {
	r := httptest.NewRequest(http.MethodPost, "/users", nil)
	if referer != "" {
		r.Header.Set("Referer", referer)
	}
	return r
}

func TestFromDocs(t *testing.T) {
	cases := []struct {
		name    string
		referer string
		prefix  string
		want    bool
	}{
		{"no referer", "", "/docs", false},
		{"docs page", "https://api.example.com/docs/", "/docs", true},
		{"bare prefix", "https://api.example.com/docs", "/docs", true},
		{"behind proxy prefix", "https://example.com/api/docs/", "/docs", true},
		{"page below the prefix", "https://api.example.com/docs/anything", "/docs", true},
		{"app page", "https://api.example.com/users", "/docs", false},
		{"lookalike prefix", "https://api.example.com/mydocs/", "/docs", false},
		{"prefix inside a word", "https://api.example.com/docsy/page", "/docs", false},
		{"relative referer", "/docs/", "/docs", true},
		{"unparseable referer", "http://%zz", "/docs", false},
		{"empty prefix falls back to /docs", "https://x.test/docs/", "", true},
		{"custom prefix match", "https://x.test/internal/api-docs/", "/api-docs", true},
		{"custom prefix mismatch", "https://x.test/docs/", "/api-docs", false},
		{"unnormalized prefix input", "https://x.test/api-docs/", "api-docs/", true},
		{"query string", "https://x.test/docs/?try=1", "/docs", true},
		{"prefix only in query", "https://x.test/users?next=/docs/", "/docs", false},
		{"prefix only in fragment", "https://x.test/users#/docs/", "/docs", false},
		{"explicit port", "https://x.test:8443/docs/", "/docs", true},
		{"ipv6 host", "http://[2001:db8::1]:8443/docs/", "/docs", true},
		{"slash prefix treated as default", "https://x.test/docs/", "/", true},
		{"slash prefix non-docs", "https://x.test/anything/", "/", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := FromDocs(reqWithReferer(tc.referer), tc.prefix); got != tc.want {
				t.Errorf("FromDocs(referer=%q, prefix=%q) = %v, want %v", tc.referer, tc.prefix, got, tc.want)
			}
		})
	}
}

func TestMuxFromDocsUsesConfiguredPrefix(t *testing.T) {
	m := New(WithTitle("T"), WithDocsPrefix("/api-docs"))
	if !m.FromDocs(reqWithReferer("https://x.test/api-docs/")) {
		t.Errorf("mux.FromDocs should match the configured prefix")
	}
	if m.FromDocs(reqWithReferer("https://x.test/docs/")) {
		t.Errorf("mux.FromDocs must not match the default prefix when reconfigured")
	}

	// Unnormalized option input normalizes consistently end to end.
	if !New(WithDocsPrefix("api-docs/")).FromDocs(reqWithReferer("https://x.test/api-docs/")) {
		t.Errorf("unnormalized WithDocsPrefix input should still match")
	}
	// Empty option input falls back to the default prefix.
	if !New(WithDocsPrefix("")).FromDocs(reqWithReferer("https://x.test/docs/")) {
		t.Errorf("empty WithDocsPrefix should fall back to /docs")
	}
	// Multi-segment prefixes match behind proxy path prefixes, and the
	// last segment alone does not.
	multi := New(WithDocsPrefix("/internal/docs"))
	if !multi.FromDocs(reqWithReferer("https://proxy.test/extra/internal/docs/")) {
		t.Errorf("multi-segment prefix should match behind a proxy prefix")
	}
	if multi.FromDocs(reqWithReferer("https://x.test/docs/")) {
		t.Errorf("last segment alone must not match a multi-segment prefix")
	}
}

// The documented guard pattern end to end: a docs-originated write is
// rejected, a regular client write and a docs-originated read pass.
func TestFromDocsGuardMiddleware(t *testing.T) {
	mux := New(WithTitle("T"))
	mux.HandleFunc("POST /users", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	})
	mux.HandleFunc("GET /users", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("[]"))
	})
	mux.Mount()

	guard := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet && mux.FromDocs(r) {
				http.Error(w, "try-it requests cannot modify data", http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
	srv := guard(mux)

	post := httptest.NewRequest(http.MethodPost, "/users", nil)
	post.Header.Set("Referer", "https://api.example.com/docs/")
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, post)
	if rr.Code != http.StatusForbidden {
		t.Errorf("docs-originated POST = %d, want 403", rr.Code)
	}

	rr = httptest.NewRecorder()
	srv.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/users", nil))
	if rr.Code != http.StatusCreated {
		t.Errorf("regular POST = %d, want 201", rr.Code)
	}

	get := httptest.NewRequest(http.MethodGet, "/users", nil)
	get.Header.Set("Referer", "https://api.example.com/docs/")
	rr = httptest.NewRecorder()
	srv.ServeHTTP(rr, get)
	if rr.Code != http.StatusOK {
		t.Errorf("docs-originated GET = %d, want 200", rr.Code)
	}
}
