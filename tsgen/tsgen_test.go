package tsgen

import (
	"bytes"
	"encoding/json"
	"flag"
	"net/http"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/FumingPower3925/stdocs"
)

var update = flag.Bool("update", false, "rewrite the golden file")

// The corpus exercises every mapping row the emitter has: scalars and
// formats, nullability in all three spellings, enums, maps, arrays of
// unions, recursion, untypeable fields, multipart, raw bodies,
// parameters in every location, security and the auto-401, mux-level
// defaults, webhooks, generics, name collisions, quoting, and JSDoc
// sanitization.

type CorpusStatus string

type CorpusTask struct {
	ID       string          `json:"id" doc:"Stable identifier"`
	Title    string          `json:"title" minLength:"1" maxLength:"200" pattern:"^.+$"`
	Status   string          `json:"status" enum:"pending,active,done" default:"pending"`
	Level    int             `json:"level" enum:"1,2,3"`
	Priority int             `json:"priority" minimum:"1" maximum:"5" example:"2"`
	Ratio    float64         `json:"ratio" exclusiveMinimum:"0" exclusiveMaximum:"1.5"`
	DueAt    *string         `json:"due_at" required:"true" format:"date-time"`
	Note     string          `json:"note" openapi:"nullable"`
	Blob     []byte          `json:"blob,omitempty"`
	Raw      json.RawMessage `json:"raw,omitempty"`
	Schema   json.RawMessage `json:"config_schema,omitempty" openapi:"schema=json-schema" doc:"Backend-driven form schema" example:"{\"type\":\"object\",\"properties\":{\"host\":{\"type\":\"string\"}}}"`
	Anything any             `json:"anything,omitempty"`
	Count    int64           `json:"count,string"`
	WeirdKey string          `json:"weird-key,omitempty" doc:"Hyphens need quoting */ and comments need escaping"`
	Parent   *CorpusTask     `json:"parent,omitempty"`
	Labels   map[string]int  `json:"labels,omitempty"`
	Tags     []string        `json:"tags,omitempty" uniqueItems:"true" minItems:"1"`
	Severity []string        `json:"severity,omitempty" enum:"info,low,high" doc:"Element enums render as a union array"`
	Links    []*CorpusRef    `json:"links,omitempty"`
}

type CorpusRef struct {
	Kind string `json:"kind"`
}

func (CorpusRef) SchemaName() string { return "NamedRef" }

type CorpusPage[T any] struct {
	Items  []T    `json:"items" required:"true"`
	Cursor string `json:"cursor,omitempty"`
}

type CorpusEmpty struct{}

type CorpusError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type CorpusListParams struct {
	Cursor   string   `query:"cursor" doc:"Opaque cursor"`
	Limit    int      `query:"limit" default:"20" minimum:"1" maximum:"100"`
	Severity []string `query:"severity" enum:"info,low,high" doc:"Repeated filter: ?severity=high&severity=low"`
	Trace    string   `header:"X-Trace-Id"`
	Theme    string   `cookie:"theme"`
}

type CorpusEvent struct {
	Kind string `json:"kind" enum:"created,deleted"`
}

func noop(http.ResponseWriter, *http.Request) {}

func corpusMux() *stdocs.Mux {
	mux := stdocs.New(
		stdocs.WithTitle("Corpus API"),
		stdocs.WithAPIVersion("1.2.3"),
		stdocs.WithVersion(stdocs.OpenAPI31),
		stdocs.WithBearerAuth("bearerAuth", "JWT"),
		stdocs.WithDefaultResponse(500, CorpusError{}),
		stdocs.WithDefaultRawResponse(503, "text/plain"),
		stdocs.WithWebhooks(map[string]stdocs.Webhook{
			"task-event": {
				Method:      "POST",
				Summary:     "Task lifecycle event",
				RequestBody: &stdocs.RequestBody{Required: true, BodyValue: CorpusEvent{}},
				Responses:   map[string]*stdocs.Response{"200": {Description: "Ack"}},
			},
		}),
	)
	mux.HandleFunc("GET /tasks", noop,
		stdocs.Summary("List tasks"),
		stdocs.WithParams(CorpusListParams{}),
		stdocs.WithResponse(200, CorpusPage[CorpusTask]{}))
	mux.HandleFunc("POST /tasks", noop,
		stdocs.Summary("Create a task"),
		stdocs.Description("The long-form description survives into JSDoc."),
		stdocs.WithBody(CorpusTask{}),
		stdocs.WithResponse(201, CorpusTask{}),
		stdocs.WithResponse(422, CorpusError{}),
		stdocs.WithSecurity("bearerAuth"))
	mux.HandleFunc("GET /tasks/{id}", noop,
		stdocs.Summary("Get one task"),
		stdocs.WithResponse(200, CorpusTask{}),
		stdocs.WithResponse(404, nil))
	mux.HandleFunc("DELETE /tasks/{id}", noop,
		stdocs.Summary("Delete a task"),
		stdocs.Deprecated(),
		stdocs.WithResponse(204, nil))
	mux.HandleFunc("GET /export", noop,
		stdocs.Summary("Export CSV"),
		stdocs.WithRawResponse(200, "text/csv"),
		stdocs.WithResponseHeader(200, "Content-Disposition", "string", "Suggested download filename"))
	mux.HandleFunc("POST /import", noop,
		stdocs.Summary("Import an archive"),
		stdocs.WithMultipartBody(
			stdocs.FilePart("archive", "The archive file"),
			stdocs.FieldPart("note", "string", "Free-form note"),
		),
		stdocs.WithResponse(202, nil))
	mux.HandleFunc("GET /empty", noop,
		stdocs.Summary("Closed empty object"),
		stdocs.WithResponse(200, CorpusEmpty{}))
	mux.HandleFunc("/ping", noop, stdocs.Summary("Method-less ping"))
	return mux
}

func TestGolden(t *testing.T) {
	got, err := Generate(corpusMux())
	if err != nil {
		t.Fatal(err)
	}
	const golden = "testdata/corpus.ts"
	if *update {
		if werr := os.WriteFile(golden, got, 0o644); werr != nil {
			t.Fatal(werr)
		}
	}
	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatalf("%v (run go test ./tsgen -update to create it)", err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("generated TypeScript differs from %s; run go test ./tsgen -update and review the diff", golden)
	}
}

func TestDeterminism(t *testing.T) {
	mux := corpusMux()
	a, err := Generate(mux)
	if err != nil {
		t.Fatal(err)
	}
	mux.Refresh()
	b, err := Generate(mux)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(a, b) {
		t.Errorf("rebuild changed the output")
	}
	c, err := Generate(corpusMux())
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(a, c) {
		t.Errorf("a fresh identical mux produced different output")
	}
}

func TestGenerateErrors(t *testing.T) {
	if _, err := Generate(nil); err == nil {
		t.Errorf("nil mux must error")
	}
	broken := stdocs.New(stdocs.WithTitle("B"), stdocs.WithOpenAPI(func(doc map[string]any) {
		doc["security"] = []any{map[string]any{"ghost": []any{}}}
	}))
	broken.HandleFunc("GET /x", noop, stdocs.Summary("X"))
	if _, err := Generate(broken); err == nil {
		t.Errorf("a document that fails validation must fail generation")
	}
}

// Late registrations rebuild automatically, like JSON.
func TestLateRoutes(t *testing.T) {
	mux := corpusMux()
	before, err := Generate(mux)
	if err != nil {
		t.Fatal(err)
	}
	mux.HandleFunc("GET /late", noop, stdocs.Summary("Late"), stdocs.WithResponse(200, CorpusError{}))
	after, err := Generate(mux)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(before, after) || !bytes.Contains(after, []byte("get_late")) {
		t.Errorf("late routes must appear on the next generation")
	}
}

// v0.6.0 verification: emission runs under the build lock — the
// model holds live operation pointers that rebuilds mutate.
func TestConcurrentGenerate(t *testing.T) {
	mux := corpusMux()
	var wg sync.WaitGroup
	for range 4 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 25 {
				if _, err := Generate(mux); err != nil {
					t.Error(err)
					return
				}
			}
		}()
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		for range 25 {
			mux.Refresh()
			mux.JSON()
		}
	}()
	wg.Wait()
}

// A component named like the glue interfaces would declaration-merge
// into a type no wire value satisfies; tsc accepts the merge
// silently, so generation refuses it with the remedy instead.
func TestReservedComponentNames(t *testing.T) {
	for _, body := range []any{makeComponents(), makeClass()} {
		mux := stdocs.New(stdocs.WithTitle("T"))
		mux.HandleFunc("POST /x", noop, stdocs.Summary("X"), stdocs.WithBody(body))
		_, err := Generate(mux)
		if err == nil || !strings.Contains(err.Error(), "SchemaName") {
			t.Errorf("%T: err = %v, want the reserved-name error with the remedy", body, err)
		}
		if _, jerr := mux.JSON(); jerr != nil {
			t.Errorf("the OpenAPI document itself stays valid: %v", jerr)
		}
	}
}

type components struct {
	X string `json:"x"`
}

type class struct {
	Y string `json:"y"`
}

func makeComponents() any { return components{} }
func makeClass() any      { return class{} }

// The TypeScript is a view of the served document: a 3.0 mux cannot
// carry webhooks, so neither does its generated module.
func TestWebhooks30Parity(t *testing.T) {
	build := func(v stdocs.SpecVersion) *stdocs.Mux {
		mux := stdocs.New(stdocs.WithTitle("T"), stdocs.WithVersion(v),
			stdocs.WithWebhooks(map[string]stdocs.Webhook{
				"task-event": {Method: "POST",
					RequestBody: &stdocs.RequestBody{Required: true, BodyValue: CorpusEvent{}},
					Responses:   map[string]*stdocs.Response{"200": {Description: "Ack"}}},
			}))
		mux.HandleFunc("GET /x", noop, stdocs.Summary("X"))
		return mux
	}
	src30, err := Generate(build(stdocs.OpenAPI30))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(src30, []byte("webhooks")) || bytes.Contains(src30, []byte("CorpusEvent")) {
		t.Errorf("a 3.0 module must not advertise webhooks the document omits")
	}
	doc30, err := build(stdocs.OpenAPI30).JSON()
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(doc30, []byte("CorpusEvent")) {
		t.Errorf("the 3.0 document must not carry orphan webhook payload components")
	}
	src31, err := Generate(build(stdocs.OpenAPI31))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(src31, []byte("webhooks")) || !bytes.Contains(src31, []byte("CorpusEvent")) {
		t.Errorf("a 3.1 module carries its webhooks")
	}
}
