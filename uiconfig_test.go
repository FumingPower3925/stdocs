package stdocs_test

// Tests for the per-UI WithConfiguration pass-through (issue #101): the
// configuration carrier appears only when a config is supplied, renders
// correctly, and never introduces an unpinned inline script.

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"html"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/FumingPower3925/stdocs"
	"github.com/FumingPower3925/stdocs/ui/redoc"
	"github.com/FumingPower3925/stdocs/ui/redocemb"
	"github.com/FumingPower3925/stdocs/ui/scalar"
	"github.com/FumingPower3925/stdocs/ui/scalaremb"
	"github.com/FumingPower3925/stdocs/ui/stoplight"
	"github.com/FumingPower3925/stdocs/ui/stoplightemb"
	"github.com/FumingPower3925/stdocs/ui/swaggerui"
	"github.com/FumingPower3925/stdocs/ui/swaggeruiemb"
)

// renderDocsPage builds a mux with the given UI option and returns the
// rendered /docs/ HTML.
func renderDocsPage(t *testing.T, opt stdocs.Option) string {
	t.Helper()
	mux := stdocs.New(stdocs.WithTitle("X"), opt)
	mux.HandleFunc("GET /x", func(w http.ResponseWriter, r *http.Request) {})
	mux.Mount()
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("GET", "/docs/", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("docs page status = %d, want 200", rec.Code)
	}
	return rec.Body.String()
}

func uiConfigCases() []struct {
	name        string
	withConfig  stdocs.Option
	without     stdocs.Option
	marker      string
	hasDefaults bool
} {
	cfgScalar := map[string]any{"theme": "purple"}
	cfgSwagger := map[string]any{"docExpansion": "none"}
	cfgRedoc := map[string]any{"hideDownloadButton": true}
	cfgStoplight := map[string]any{"hideTryItPanel": "true"}
	return []struct {
		name        string
		withConfig  stdocs.Option
		without     stdocs.Option
		marker      string
		hasDefaults bool // Scalar/Swagger ship CSP-safe defaults, so the carrier is always present
	}{
		{"scalar", scalar.WithUI(scalar.WithConfiguration(cfgScalar)), scalar.WithUI(), "data-configuration=", true},
		{"scalaremb", scalaremb.WithUI(scalaremb.WithConfiguration(cfgScalar)), scalaremb.WithUI(), "data-configuration=", true},
		{"swaggerui", swaggerui.WithUI(swaggerui.WithConfiguration(cfgSwagger)), swaggerui.WithUI(), `id="swagger-config"`, true},
		{"swaggeruiemb", swaggeruiemb.WithUI(swaggeruiemb.WithConfiguration(cfgSwagger)), swaggeruiemb.WithUI(), `id="swagger-config"`, true},
		{"redoc", redoc.WithUI(redoc.WithConfiguration(cfgRedoc)), redoc.WithUI(), `id="redoc-config"`, false},
		{"redocemb", redocemb.WithUI(redocemb.WithConfiguration(cfgRedoc)), redocemb.WithUI(), `id="redoc-config"`, false},
		{"stoplight", stoplight.WithUI(stoplight.WithConfiguration(cfgStoplight)), stoplight.WithUI(), `hideTryItPanel="true"`, false},
		{"stoplightemb", stoplightemb.WithUI(stoplightemb.WithConfiguration(cfgStoplight)), stoplightemb.WithUI(), `hideTryItPanel="true"`, false},
	}
}

func TestUIConfigCarrierPresence(t *testing.T) {
	for _, c := range uiConfigCases() {
		t.Run(c.name, func(t *testing.T) {
			with := renderDocsPage(t, c.withConfig)
			if !strings.Contains(with, c.marker) {
				t.Errorf("with config: page missing marker %q", c.marker)
			}
			without := renderDocsPage(t, c.without)
			gotWithout := strings.Contains(without, c.marker)
			switch {
			case c.hasDefaults && !gotWithout:
				// Scalar/Swagger always render a carrier for their CSP-safe
				// defaults, so the carrier marker must be present even with
				// no caller config. (The per-UI config marker for these is
				// the carrier element itself, which the defaults populate.)
				t.Errorf("no config: page missing default-config carrier %q", c.marker)
			case !c.hasDefaults && gotWithout:
				t.Errorf("no config: page unexpectedly contains marker %q (carrier must be absent without defaults or caller config)", c.marker)
			}
		})
	}
}

// TestUICSPSafeDefaults verifies that Scalar and Swagger ship CSP-safe
// defaults out of the box, that a caller's WithConfiguration overrides
// them key-by-key, and that UIs without defaults stay nil when given no
// config.
func TestUICSPSafeDefaults(t *testing.T) {
	// Scalar: defaults disable the phone-home chrome.
	var c stdocs.Config
	scalar.WithUI()(&c)
	if c.UIConfig["showDeveloperTools"] != "never" {
		t.Errorf("scalar default showDeveloperTools = %v, want never", c.UIConfig["showDeveloperTools"])
	}
	if c.UIConfig["withDefaultFonts"] != false {
		t.Errorf("scalar default withDefaultFonts = %v, want false", c.UIConfig["withDefaultFonts"])
	}
	if ag, _ := c.UIConfig["agent"].(map[string]any); ag["disabled"] != true {
		t.Errorf("scalar default agent = %v, want disabled", c.UIConfig["agent"])
	}

	// Caller override wins on its key; untouched defaults remain.
	var c2 stdocs.Config
	scalar.WithUI(scalar.WithConfiguration(map[string]any{"agent": map[string]any{"disabled": false}, "theme": "purple"}))(&c2)
	if ag, _ := c2.UIConfig["agent"].(map[string]any); ag["disabled"] != false {
		t.Errorf("override agent = %v, want re-enabled", c2.UIConfig["agent"])
	}
	if mcp, _ := c2.UIConfig["mcp"].(map[string]any); mcp["disabled"] != true {
		t.Errorf("untouched default mcp = %v, want still disabled", c2.UIConfig["mcp"])
	}
	if c2.UIConfig["theme"] != "purple" {
		t.Errorf("caller theme = %v, want purple", c2.UIConfig["theme"])
	}

	// Swagger: validatorUrl defaulted off, overridable.
	var c3 stdocs.Config
	swaggerui.WithUI()(&c3)
	if v, ok := c3.UIConfig["validatorUrl"]; !ok || v != nil {
		t.Errorf("swagger default validatorUrl = %v (present=%v), want nil", v, ok)
	}
	var c4 stdocs.Config
	swaggerui.WithUI(swaggerui.WithConfiguration(map[string]any{"validatorUrl": "https://example/validator"}))(&c4)
	if c4.UIConfig["validatorUrl"] != "https://example/validator" {
		t.Errorf("override validatorUrl = %v", c4.UIConfig["validatorUrl"])
	}

	// Redoc has no defaults: nil config when none supplied.
	var c5 stdocs.Config
	redoc.WithUI()(&c5)
	if c5.UIConfig != nil {
		t.Errorf("redoc no-config UIConfig = %v, want nil", c5.UIConfig)
	}
}

// TestScalarConfigRoundTrip proves the JSON placed in data-configuration
// decodes (HTML entities) and parses back to the exact config map, so
// Scalar receives valid configuration.
func TestScalarConfigRoundTrip(t *testing.T) {
	cfg := map[string]any{"theme": "purple", "layout": "modern", "label": `it's "x" <b>`}
	page := renderDocsPage(t, scalar.WithUI(scalar.WithConfiguration(cfg)))
	const attr = `data-configuration="`
	i := strings.Index(page, attr)
	if i < 0 {
		t.Fatal("data-configuration attribute not found")
	}
	rest := page[i+len(attr):]
	raw := rest[:strings.IndexByte(rest, '"')]
	var got map[string]any
	if err := json.Unmarshal([]byte(html.UnescapeString(raw)), &got); err != nil {
		t.Fatalf("data-configuration did not decode to JSON: %v", err)
	}
	for k, v := range cfg {
		if got[k] != v {
			t.Errorf("decoded[%q] = %v, want %v", k, got[k], v)
		}
	}
}

// TestRedocBaselineUsesInit documents the one intentional baseline
// change: Redoc now boots via Redoc.init rather than the <redoc spec-url>
// auto-mount element, even with no config.
func TestRedocBaselineUsesInit(t *testing.T) {
	page := renderDocsPage(t, redoc.WithUI())
	if !strings.Contains(page, "Redoc.init(") {
		t.Error("Redoc page should boot via Redoc.init(")
	}
	if strings.Contains(page, "<redoc spec-url") {
		t.Error("Redoc page should no longer use the <redoc spec-url> auto-mount element")
	}
}

// TestPerUICSPParityWithConfig re-runs the CSP parity guard with a config
// supplied, proving the JSON carrier stays non-executable (no new hash
// demanded) and the static init hashes still match. It reuses
// execInlineScripts and directive from cspparity_test.go.
func TestPerUICSPParityWithConfig(t *testing.T) {
	for _, c := range uiConfigCases() {
		t.Run(c.name, func(t *testing.T) {
			mux := stdocs.New(stdocs.WithTitle("X"), c.withConfig)
			mux.HandleFunc("GET /x", func(w http.ResponseWriter, r *http.Request) {})
			mux.Mount()
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, httptest.NewRequest("GET", "/docs/", nil))

			csp := rec.Header().Get("Content-Security-Policy")
			scriptSrc, ok := directive(csp, "script-src")
			if !ok {
				t.Fatal("CSP has no script-src directive")
			}
			if strings.Contains(scriptSrc, "'unsafe-inline'") {
				t.Errorf("script-src allows 'unsafe-inline': %q", scriptSrc)
			}
			for _, script := range execInlineScripts(rec.Body.Bytes()) {
				sum := sha256.Sum256(script)
				h := "'sha256-" + base64.StdEncoding.EncodeToString(sum[:]) + "'"
				if !strings.Contains(scriptSrc, h) {
					t.Errorf("inline script hash %s not pinned in script-src %q", h, scriptSrc)
				}
			}
		})
	}
}

// TestConfigInjectionNeutralized feeds each UI a hostile config (a
// </script> breakout, quotes, angle brackets, ampersands, and a {{ }}
// sequence; for Stoplight also a hostile attribute name and non-string
// values) and asserts the served page neither executes the breakout nor
// produces an unpinned executable inline script. This makes the
// no-injection guarantee an active test rather than a latent one.
func TestConfigInjectionNeutralized(t *testing.T) {
	evil := `</script><script>alert(1)</script>"'<>&{{.Title}}`
	cases := []struct {
		name string
		opt  stdocs.Option
	}{
		{"scalar", scalar.WithUI(scalar.WithConfiguration(map[string]any{"injected": evil}))},
		{"swaggerui", swaggerui.WithUI(swaggerui.WithConfiguration(map[string]any{"injected": evil}))},
		{"redoc", redoc.WithUI(redoc.WithConfiguration(map[string]any{"injected": evil}))},
		{"stoplight", stoplight.WithUI(stoplight.WithConfiguration(map[string]any{"injected": evil, "</bad attr>": "z", "flag": true, "n": 5}))},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			mux := stdocs.New(stdocs.WithTitle("X"), c.opt)
			mux.HandleFunc("GET /x", func(w http.ResponseWriter, r *http.Request) {})
			mux.Mount()
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, httptest.NewRequest("GET", "/docs/", nil))
			body := rec.Body.String()
			if strings.Contains(body, "<script>alert(1)</script>") {
				t.Errorf("%s: executable breakout present in served page", c.name)
			}
			if strings.Contains(body, "</bad") {
				t.Errorf("%s: hostile attribute name leaked into the page", c.name)
			}
			csp := rec.Header().Get("Content-Security-Policy")
			scriptSrc, _ := directive(csp, "script-src")
			if strings.Contains(scriptSrc, "'unsafe-inline'") {
				t.Errorf("%s: script-src allows 'unsafe-inline'", c.name)
			}
			for _, script := range execInlineScripts(rec.Body.Bytes()) {
				sum := sha256.Sum256(script)
				h := "'sha256-" + base64.StdEncoding.EncodeToString(sum[:]) + "'"
				if !strings.Contains(scriptSrc, h) {
					t.Errorf("%s: config injection produced an unpinned executable inline script", c.name)
				}
			}
		})
	}
	// The Stoplight non-string branch renders bool/number as attributes.
	body := renderDocsPage(t, stoplight.WithUI(stoplight.WithConfiguration(map[string]any{"flag": true, "n": 5})))
	if !strings.Contains(body, `flag="true"`) || !strings.Contains(body, `n="5"`) {
		t.Errorf("stoplight non-string values not rendered as attributes: %q", body[strings.Index(body, "<elements-api"):strings.Index(body, "</elements-api>")+1])
	}
}

// TestJSONCarrierRoundTrip proves the Swagger UI and Redoc JSON carriers
// (template.JS in a <script type="application/json"> block) emit valid,
// parseable JSON that round-trips a value containing JSON/HTML-special
// characters.
func TestJSONCarrierRoundTrip(t *testing.T) {
	cfg := map[string]any{"docExpansion": "none", "deepLinking": false, "k": `a"b<c>&d`}
	cases := []struct {
		name, id string
		opt      stdocs.Option
	}{
		{"swaggerui", "swagger-config", swaggerui.WithUI(swaggerui.WithConfiguration(cfg))},
		{"redoc", "redoc-config", redoc.WithUI(redoc.WithConfiguration(cfg))},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			page := renderDocsPage(t, c.opt)
			open := `<script id="` + c.id + `" type="application/json">`
			_, after, ok := strings.Cut(page, open)
			if !ok {
				t.Fatalf("carrier %q not found", c.id)
			}
			raw, _, ok := strings.Cut(after, "</script>")
			if !ok {
				t.Fatalf("carrier %q not closed", c.id)
			}
			var got map[string]any
			if err := json.Unmarshal([]byte(raw), &got); err != nil {
				t.Fatalf("carrier JSON did not parse: %v (raw=%q)", err, raw)
			}
			if got["k"] != `a"b<c>&d` {
				t.Errorf("k round-trip = %v, want a\"b<c>&d", got["k"])
			}
		})
	}
}
