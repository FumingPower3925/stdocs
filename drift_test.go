package stdocs

import (
	"bytes"
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

// v0.4.1: DriftWarn is race-free against Refresh, tracks late
// registrations, and applies the content-type check to
// default-covered statuses.
func TestDriftWarnSnapshotSemantics(t *testing.T) {
	type Envelope struct {
		Message string `json:"message"`
	}
	mux := New(WithTitle("T"), WithDefaultResponse(0, Envelope{}))
	mux.HandleFunc("GET /a", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "bad", http.StatusBadRequest) // text/plain under a JSON default: drift
	}, Summary("A"))
	logf, warnings := collectWarnings()
	h := DriftWarn(mux, logf)
	driftGet(h, "/a")
	got := warnings()
	if len(got) != 1 || !strings.Contains(got[0], "text/plain") {
		t.Errorf("JSON-documented default served as text/plain must warn: %q", got)
	}

	// Late registration: tracked after the snapshot refreshes.
	mux.HandleFunc("GET /late", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusTeapot) // covered by default, JSON body: no warning
		w.Write([]byte(`{"message":"hi"}`))
	}, Summary("Late"))
	driftGet(h, "/late")
	if len(warnings()) != 1 {
		t.Errorf("late JSON-clean route should add no warnings: %q", warnings())
	}

	// Race: concurrent traffic + Refresh under -race.
	var wg sync.WaitGroup
	for range 6 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 20 {
				driftGet(h, "/late")
				mux.Refresh()
				mux.JSON()
			}
		}()
	}
	wg.Wait()
}

// v0.4.2: body sampling — missing required keys and undocumented
// extras warn once each; clean handlers, capped bodies, and
// non-object schemas stay quiet.
func TestDriftSampleBodies(t *testing.T) {
	type Task struct {
		ID   string `json:"id"`
		Name string `json:"name,omitempty"`
	}
	type Envelope struct {
		Message string `json:"message"`
	}
	mux := New(WithTitle("T"), WithDefaultResponse(0, Envelope{}))
	mux.HandleFunc("GET /missing", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"name":"x"}`)) // id (required) absent
	}, Summary("M"), WithResponse(200, Task{}))
	mux.HandleFunc("GET /extra", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"id":"1","surprise":true}`))
	}, Summary("E"), WithResponse(200, Task{}))
	mux.HandleFunc("GET /clean", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"id":"1","name":"ok"}`))
	}, Summary("C"), WithResponse(200, Task{}))
	mux.HandleFunc("GET /boom", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadGateway)
		w.Write([]byte(`{"wrong":"shape"}`)) // default entry's shape applies
	}, Summary("B"), WithResponse(200, Task{}))

	logf, warnings := collectWarnings()
	h := DriftWarn(mux, logf, DriftSampleBodies())
	for _, p := range []string{"/missing", "/missing", "/extra", "/clean", "/boom"} {
		driftGet(h, p)
	}
	got := strings.Join(warnings(), "\n")
	if c := strings.Count(got, `missing required field "id"`); c != 1 {
		t.Errorf("want exactly one missing-required warning for /missing, got %d in:\n%s", c, got)
	}
	if !strings.Contains(got, "/extra") || !strings.Contains(got, "surprise") {
		t.Errorf("undocumented key must warn with the key named:\n%s", got)
	}
	if strings.Contains(got, "/clean") {
		t.Errorf("clean handler must not warn:\n%s", got)
	}
	if !strings.Contains(got, `response 502 is missing required field "message"`) {
		t.Errorf("default-entry shape must apply to undeclared statuses:\n%s", got)
	}

	// Without the opt: no body warnings at all.
	logq, quiet := collectWarnings()
	hq := DriftWarn(mux, logq)
	driftGet(hq, "/missing")
	for _, w := range quiet() {
		if strings.Contains(w, "required field") {
			t.Errorf("sampling must be opt-in: %s", w)
		}
	}

	// Oversized bodies are skipped, not mis-parsed.
	big := New(WithTitle("T"))
	big.HandleFunc("GET /big", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"id":"`))
		w.Write(bytes.Repeat([]byte("x"), 70<<10))
		w.Write([]byte(`"}`))
	}, Summary("Big"), WithResponse(200, Task{}))
	logb, bigWarnings := collectWarnings()
	hb := DriftWarn(big, logb, DriftSampleBodies())
	driftGet(hb, "/big")
	for _, w := range bigWarnings() {
		if strings.Contains(w, "required field") {
			t.Errorf("overflowing bodies must be skipped: %s", w)
		}
	}
}
