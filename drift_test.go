package stdocs

import (
	"bytes"
	"fmt"
	"io"
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

// v0.4.2 user-sim finding: declared non-JSON content types are
// contracts too.
func TestDriftRawContentTypeContract(t *testing.T) {
	mux := New(WithTitle("T"))
	mux.HandleFunc("GET /csv-wrong", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte("a,b\n"))
	}, Summary("W"), WithRawResponse(200, "text/csv"))
	mux.HandleFunc("GET /csv-right", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
		w.Write([]byte("a,b\n"))
	}, Summary("R"), WithRawResponse(200, "text/csv"))
	logf, warnings := collectWarnings()
	h := DriftWarn(mux, logf)
	driftGet(h, "/csv-wrong")
	driftGet(h, "/csv-wrong")
	driftGet(h, "/csv-right")
	got := warnings()
	if len(got) != 1 || !strings.Contains(got[0], "text/plain") || !strings.Contains(got[0], "text/csv") {
		t.Errorf("declared CSV served as plain must warn exactly once: %q", got)
	}
}

func TestDriftWarnObservedViaQualifier(t *testing.T) {
	mux := New(WithTitle("T"))
	mux.HandleFunc("POST /qualified", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusConflict)
	}, Summary("Q"))
	mux.HandleFunc("/any", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusConflict)
	}, Summary("A"))
	logf, warnings := collectWarnings()
	h := DriftWarn(mux, logf)
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/qualified", nil))
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPut, "/any", nil))
	got := warnings()
	if len(got) != 2 {
		t.Fatalf("warnings = %d (%q), want 2", len(got), got)
	}
	for _, w := range got {
		switch {
		case strings.Contains(w, "POST /qualified"):
			// The pattern already names the method; repeating it is noise.
			if strings.Contains(w, "observed via") {
				t.Errorf("method-qualified warning repeats the method: %q", w)
			}
		case strings.Contains(w, "/any"):
			if !strings.Contains(w, "(observed via PUT)") {
				t.Errorf("method-less warning lost the observed method: %q", w)
			}
		default:
			t.Errorf("unexpected warning %q", w)
		}
	}
}

func TestDriftNotify(t *testing.T) {
	type Task struct {
		ID   string `json:"id" required:"true"`
		Name string `json:"name"`
	}
	mux := New(WithTitle("T"))
	mux.HandleFunc("GET /tasks", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"name":"a","secret":true}`)) // id missing, secret extra
	}, Summary("L"), WithResponse(200, Task{}))
	mux.HandleFunc("GET /surprise", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	}, Summary("S"))
	mux.HandleFunc("GET /wrongct", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte("plain"))
	}, Summary("C"), WithResponse(200, Task{}))

	var mu sync.Mutex
	var found []DriftFinding
	var second int
	logf, warnings := collectWarnings()
	h := DriftWarn(mux, logf,
		DriftSampleBodies(),
		DriftNotify(func(f DriftFinding) {
			mu.Lock()
			defer mu.Unlock()
			found = append(found, f)
		}),
		DriftNotify(func(DriftFinding) { mu.Lock(); second++; mu.Unlock() }),
	)
	driftGet(h, "/tasks")
	driftGet(h, "/tasks") // deduplicated: no second delivery
	driftGet(h, "/surprise")
	driftGet(h, "/wrongct")

	mu.Lock()
	defer mu.Unlock()
	byCode := map[string]DriftFinding{}
	for _, f := range found {
		byCode[f.Code] = f
	}
	if len(found) != 4 || len(byCode) != 4 {
		t.Fatalf("findings = %+v, want 4 distinct codes", found)
	}
	if second != 4 {
		t.Errorf("second callback got %d deliveries, want 4", second)
	}
	if f := byCode["missing-required-field"]; f.Route != "GET /tasks" || f.Status != 200 || f.Field != "id" {
		t.Errorf("missing-required-field = %+v", f)
	}
	if f := byCode["undocumented-fields"]; f.Route != "GET /tasks" || f.Status != 200 || f.Field != "" {
		t.Errorf("undocumented-fields = %+v", f)
	}
	if f := byCode["undeclared-status"]; f.Route != "GET /surprise" || f.Status != 418 {
		t.Errorf("undeclared-status = %+v", f)
	}
	if f := byCode["content-type-mismatch"]; f.Route != "GET /wrongct" || f.Status != 200 {
		t.Errorf("content-type-mismatch = %+v", f)
	}
	// Each Message matches its log line minus the prefix, so gates and
	// logs never tell different stories.
	got := warnings()
	if len(got) != 4 {
		t.Fatalf("log lines = %d (%q), want 4", len(got), got)
	}
	for _, line := range got {
		rest, ok := strings.CutPrefix(line, "stdocs drift: ")
		if !ok {
			t.Fatalf("log line %q lacks the prefix", line)
		}
		matched := false
		for _, f := range found {
			if f.Message == rest {
				matched = true
			}
		}
		if !matched {
			t.Errorf("log line %q has no matching finding message", line)
		}
	}
}

func TestDriftNotifyBuildFailed(t *testing.T) {
	mux := New(WithTitle("T"), WithOpenAPI(func(doc map[string]any) {
		// An out-of-band edit referencing an unregistered scheme is the
		// one way a build fails after registration-time validation.
		doc["security"] = []any{map[string]any{"ghost": []any{}}}
	}))
	mux.HandleFunc("GET /x", func(w http.ResponseWriter, r *http.Request) {}, Summary("X"))

	var mu sync.Mutex
	var found []DriftFinding
	logf, warnings := collectWarnings()
	h := DriftWarn(mux, logf, DriftNotify(func(f DriftFinding) {
		// Callbacks run outside the build lock, so reaching back into
		// the document — the natural move for a gate that wants Lint
		// findings next to drift findings — must not deadlock.
		mux.Lint()
		mu.Lock()
		defer mu.Unlock()
		found = append(found, f)
	}))
	driftGet(h, "/x")

	mu.Lock()
	defer mu.Unlock()
	if len(found) != 1 || found[0].Code != "build-failed" || found[0].Route != "" || found[0].Status != 0 {
		t.Fatalf("findings = %+v, want one build-failed", found)
	}
	if !strings.Contains(found[0].Message, "ghost") {
		t.Errorf("Message = %q, want the build error", found[0].Message)
	}
	if got := warnings(); len(got) != 1 || !strings.Contains(got[0], "build failed") {
		t.Errorf("log = %q", got)
	}
}

func TestDriftSampleBodiesRows(t *testing.T) {
	type Order struct {
		ID       string `json:"id" required:"true"`
		FeeCents int    `json:"fee_cents" required:"true"`
	}
	type OrderList struct {
		Orders []Order `json:"orders" required:"true"`
		Total  int     `json:"total"`
	}
	mux := New(WithTitle("T"))
	// The wrapped-list shape that escaped top-level-only sampling:
	// some rows omit fee_cents, some carry an undocumented debug key.
	mux.HandleFunc("GET /orders", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"orders":[
			{"id":"a","fee_cents":100},
			{"id":"b","_dbg":1},
			{"id":"c","fee_cents":300,"_dbg":2},
			null
		],"total":3}`))
	}, Summary("L"), WithResponse(200, OrderList{}))
	mux.HandleFunc("GET /orders-bare", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`[{"id":"a"},{"id":"b","fee_cents":2}]`))
	}, Summary("B"), WithResponse(200, []Order{}))
	mux.HandleFunc("GET /orders-empty", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"orders":[],"total":0}`))
	}, Summary("E"), WithResponse(200, OrderList{}))
	mux.HandleFunc("GET /orders-null", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"orders":null,"total":0}`))
	}, Summary("N"), WithResponse(200, OrderList{}))

	var mu sync.Mutex
	var found []DriftFinding
	logf, warnings := collectWarnings()
	h := DriftWarn(mux, logf, DriftSampleBodies(), DriftNotify(func(f DriftFinding) {
		mu.Lock()
		defer mu.Unlock()
		found = append(found, f)
	}))
	driftGet(h, "/orders")
	driftGet(h, "/orders") // dedup: rows warn once, not per request
	driftGet(h, "/orders-bare")
	driftGet(h, "/orders-empty") // empty array proves nothing
	driftGet(h, "/orders-null")  // null array proves nothing

	mu.Lock()
	defer mu.Unlock()
	fields := make(map[string]string, len(found)) // Field -> Code
	for _, f := range found {
		fields[f.Field] = f.Code
	}
	want := map[string]string{
		"orders[].fee_cents": "missing-required-field",
		"orders[]":           "undocumented-fields",
		"[].fee_cents":       "missing-required-field",
	}
	for field, code := range want {
		if fields[field] != code {
			t.Errorf("field %q: code = %q, want %q (all: %+v)", field, fields[field], code, found)
		}
	}
	if len(found) != len(want) {
		t.Errorf("findings = %d (%+v), want %d", len(found), found, len(want))
	}
	for _, line := range warnings() {
		if strings.Contains(line, "orders[]._dbg") {
			return
		}
	}
	t.Errorf("no warning names the undocumented row key: %q", warnings())
}

func TestDriftSampleBodiesRowVolumeBounded(t *testing.T) {
	type Row struct {
		ID string `json:"id" required:"true"`
	}
	type List struct {
		Rows []Row `json:"rows" required:"true"`
	}
	mux := New(WithTitle("T"))
	mux.HandleFunc("GET /rows", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		var b strings.Builder
		b.WriteString(`{"rows":[`)
		for i := range 200 {
			if i > 0 {
				b.WriteByte(',')
			}
			fmt.Fprintf(&b, `{"x%d":1}`, i) // every row misses id, every row adds a unique extra
		}
		b.WriteString(`]}`)
		w.Write([]byte(b.String()))
	}, Summary("R"), WithResponse(200, List{}))

	logf, warnings := collectWarnings()
	h := DriftWarn(mux, logf, DriftSampleBodies())
	driftGet(h, "/rows")
	if got := warnings(); len(got) != 2 { // one missing-required, one extras digest
		t.Errorf("warnings = %d (%q), want 2 regardless of 200 rows", len(got), got)
	}
}

func TestDriftDefaultEntryContentType(t *testing.T) {
	mux := New(WithTitle("T"))
	// The default entry declares raw CSV; statuses covered only by it
	// inherit that media-type contract.
	mux.HandleFunc("GET /feed", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusBadGateway)
		w.Write([]byte("oops"))
	}, Summary("F"), WithRawResponse(0, "text/csv"))
	mux.HandleFunc("GET /feed-ok", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
		w.WriteHeader(http.StatusBadGateway)
		w.Write([]byte("a,b\n"))
	}, Summary("G"), WithRawResponse(0, "text/csv"))

	logf, warnings := collectWarnings()
	h := DriftWarn(mux, logf)
	driftGet(h, "/feed")
	driftGet(h, "/feed")
	driftGet(h, "/feed-ok")
	got := warnings()
	if len(got) != 1 || !strings.Contains(got[0], "text/plain") || !strings.Contains(got[0], "text/csv") {
		t.Errorf("default-entry CSV served as plain must warn exactly once: %q", got)
	}
}

func TestDriftSampleBodiesNullBody(t *testing.T) {
	type Task struct {
		ID string `json:"id" required:"true"`
	}
	mux := New(WithTitle("T"))
	mux.HandleFunc("GET /null", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`null`))
	}, Summary("N"), WithResponse(200, Task{}))

	logf, warnings := collectWarnings()
	h := DriftWarn(mux, logf, DriftSampleBodies())
	driftGet(h, "/null")
	if got := warnings(); len(got) != 0 {
		t.Errorf("a literal null body declares nothing about fields: %q", got)
	}
}

func TestDriftSampleBodiesReadFrom(t *testing.T) {
	type Task struct {
		ID string `json:"id" required:"true"`
	}
	mux := New(WithTitle("T"))
	mux.HandleFunc("GET /streamed", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// io.Copy prefers the recorder's ReadFrom — the sendfile path
		// that used to skip the capture buffer.
		io.Copy(w, strings.NewReader(`{"name":"x"}`))
	}, Summary("S"), WithResponse(200, Task{}))

	logf, warnings := collectWarnings()
	h := DriftWarn(mux, logf, DriftSampleBodies())
	driftGet(h, "/streamed")
	got := strings.Join(warnings(), "\n")
	if !strings.Contains(got, `missing required field "id"`) || !strings.Contains(got, "name") {
		t.Errorf("bodies written via ReadFrom must be sampled: %q", got)
	}
}
