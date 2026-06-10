package stdocs

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

// collectWarnings returns a threadsafe logf and a getter for what it
// received.
func collectWarnings() (func(string, ...any), func() []string) {
	var mu sync.Mutex
	var got []string
	logf := func(format string, args ...any) {
		mu.Lock()
		defer mu.Unlock()
		got = append(got, fmt.Sprintf(format, args...))
	}
	return logf, func() []string {
		mu.Lock()
		defer mu.Unlock()
		return append([]string(nil), got...)
	}
}

func driftGet(h http.Handler, path string) {
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, path, nil))
}

func TestDriftWarnUndocumentedStatus(t *testing.T) {
	type Task struct {
		ID string `json:"id"`
	}
	mux := New(WithTitle("T"))
	mux.HandleFunc("GET /missing", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "nope", http.StatusNotFound) // 404 is not documented
	})
	mux.HandleFunc("GET /documented", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "nope", http.StatusNotFound)
	}, WithResponse(200, Task{}), WithResponse(404, nil))
	mux.Mount()

	logf, warnings := collectWarnings()
	h := DriftWarn(mux, logf)

	driftGet(h, "/missing")
	driftGet(h, "/missing") // second hit must not warn again
	driftGet(h, "/documented")
	driftGet(h, "/docs/openapi.json") // docs routes have no contract

	got := warnings()
	if len(got) != 1 {
		t.Fatalf("warnings = %d (%q), want exactly 1", len(got), got)
	}
	if !strings.Contains(got[0], "GET /missing") || !strings.Contains(got[0], "404") {
		t.Errorf("warning = %q", got[0])
	}
}

func TestDriftWarnContentType(t *testing.T) {
	type Task struct {
		ID string `json:"id"`
	}
	mux := New(WithTitle("T"))
	mux.HandleFunc("GET /csv", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/csv")
		w.Write([]byte("id\n1\n"))
	}, WithResponse(200, Task{})) // documented as JSON, served as CSV
	mux.HandleFunc("GET /json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Write([]byte(`{"id":"1"}`))
	}, WithResponse(200, Task{}))
	mux.HandleFunc("GET /nobody", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte("ok")) // auto-200 documents no body: no warning
	})

	logf, warnings := collectWarnings()
	h := DriftWarn(mux, logf)
	driftGet(h, "/csv")
	driftGet(h, "/json")
	driftGet(h, "/nobody")

	got := warnings()
	if len(got) != 1 {
		t.Fatalf("warnings = %d (%q), want exactly 1", len(got), got)
	}
	if !strings.Contains(got[0], "text/csv") {
		t.Errorf("warning = %q", got[0])
	}
}

func TestDriftWarnRespectsDefaultsAndVisibility(t *testing.T) {
	type APIError struct {
		Message string `json:"message"`
	}
	mux := New(WithTitle("T"), WithDefaultResponse(500, APIError{}))
	mux.HandleFunc("GET /boom", func(w http.ResponseWriter, r *http.Request) {
		// Covered by the mux-level default 500 — and written as the
		// JSON the document declares.
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"message":"boom"}`))
	})
	mux.HandleFunc("GET /catchall", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	}, WithResponse(0, nil)) // a default response covers any status
	mux.HandleFunc("GET /hidden", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusConflict)
	}, Hidden()) // excluded from the document: no contract

	logf, warnings := collectWarnings()
	h := DriftWarn(mux, logf)
	driftGet(h, "/boom")
	driftGet(h, "/catchall")
	driftGet(h, "/hidden")

	if got := warnings(); len(got) != 0 {
		t.Errorf("warnings = %q, want none", got)
	}
}
