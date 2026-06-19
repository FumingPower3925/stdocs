//go:build uismoke

package stdocs_test

// UI rendering smoke tests: serve a corpus API through every bundled
// UI and assert a real headless browser renders the operations —
// catching blank pages, broken asset wiring, and facet-invisible
// rendering that no Go-side test can see.
//
// Run locally (Chrome or Chromium required):
//
//	go test -tags uismoke -run TestUISmoke -v .
//
// The CDN-backed UIs fetch their bundles from the network; set
// STDOCS_UI_SMOKE_OFFLINE=1 to restrict the run to the built-in page
// and the embedded UIs.

import (
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

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

type Widget struct {
	ID       string `json:"id" doc:"Unique widget ID"`
	Priority int    `json:"priority" minimum:"1" maximum:"5" default:"3"`
	Severity string `json:"severity" enum:"low,medium,high"`
}

func corpusMux(extra ...stdocs.Option) *stdocs.Mux {
	mux := stdocs.New(append([]stdocs.Option{stdocs.WithTitle("Smoke API")}, extra...)...)
	mux.HandleFunc("GET /widgets/{id}", func(w http.ResponseWriter, r *http.Request) {},
		stdocs.Summary("Get widget by id"),
		stdocs.WithResponse(200, Widget{}),
	)
	mux.HandleFunc("POST /widgets", func(w http.ResponseWriter, r *http.Request) {},
		stdocs.Summary("Create widget"),
		stdocs.WithBody(Widget{}),
		stdocs.WithResponse(201, Widget{}),
	)
	// The harness page iframes the docs (same origin) and copies all
	// rendered text — including open shadow roots, which
	// --dump-dom cannot serialize — into the light DOM where the DOM
	// dump can see it.
	mux.HandleFunc("GET /smoketest", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(harnessHTML))
	}, stdocs.Hidden())
	mux.Mount()
	return mux
}

const harnessHTML = `<!doctype html>
<html><body>
<div id="out">PENDING</div>
<iframe id="f" src="/docs/" style="width:1400px;height:2400px"></iframe>
<script>
function allText(root) {
  // Text nodes only, skipping style/script bodies; unlike innerText
  // this includes collapsed (display:none) nav nodes, where several
  // UIs keep their operation summaries.
  let txt = "";
  const walker = document.createTreeWalker(root, NodeFilter.SHOW_TEXT, {
    acceptNode: (n) => {
      const p = n.parentElement;
      return p && (p.tagName === "STYLE" || p.tagName === "SCRIPT")
        ? NodeFilter.FILTER_REJECT : NodeFilter.FILTER_ACCEPT;
    },
  });
  while (walker.nextNode()) txt += walker.currentNode.nodeValue + "\n";
  for (const el of root.querySelectorAll("*")) {
    if (el.shadowRoot) txt += allText(el.shadowRoot);
  }
  return txt;
}
const f = document.getElementById("f");
let tries = 0;
const timer = setInterval(() => {
  tries++;
  let txt = "";
  try {
    txt = allText(f.contentDocument.body);
  } catch (e) { /* not loaded yet */ }
  if ((txt.includes("Get widget") || txt.includes("Schemas")) || tries > 60) {
    document.getElementById("out").textContent = "CAPTURED:" + txt.slice(0, 60000);
    clearInterval(timer);
  }
}, 500);
</script>
</body></html>`

func chromeBin(t *testing.T) string {
	t.Helper()
	if bin := os.Getenv("CHROME_BIN"); bin != "" {
		return bin
	}
	for _, c := range []string{
		"google-chrome", "chromium", "chromium-browser",
		"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
	} {
		if p, err := exec.LookPath(c); err == nil {
			return p
		}
	}
	t.Skip("no Chrome/Chromium found; set CHROME_BIN")
	return ""
}

// renderDOM loads url in headless Chrome and returns the settled DOM.
func renderDOM(t *testing.T, bin, url string) string {
	t.Helper()
	cmd := exec.Command(bin,
		"--headless=new", "--disable-gpu", "--no-sandbox",
		"--window-size=1440,2400",
		"--virtual-time-budget=35000", "--timeout=45000",
		// Surface CSP violations on stderr so TestUICSP can read them
		// out of the combined output alongside the DOM dump.
		"--enable-logging=stderr", "--v=1",
		"--dump-dom", url,
	)
	out, err := cmd.CombinedOutput()
	if err != nil && len(out) == 0 {
		t.Fatalf("chrome failed: %v", err)
	}
	return string(out)
}

// cspFailures returns CSP-violation log lines that signal a real policy
// break: an inline script or a required script/style/worker being
// refused. Violations for the UIs' intentional third-party phone-home
// (Scalar's fonts and registry API, Redoc's external logo) are expected
// and tolerated — the policy blocks those on purpose.
func cspFailures(dom string) []string {
	tolerated := []string{"fonts.scalar.com", "api.scalar.com", "cdn.redoc.ly"}
	var bad []string
	for _, ln := range strings.Split(dom, "\n") {
		violation := strings.Contains(ln, "violates the following Content Security Policy") ||
			strings.Contains(ln, "Refused to execute inline script") ||
			strings.Contains(ln, "Refused to load")
		if !violation {
			continue
		}
		// A refused connect under connect-src 'self' is always an
		// external call we mean to block.
		if strings.Contains(ln, "Refused to connect") || strings.Contains(ln, "Connecting to") {
			continue
		}
		ok := false
		for _, h := range tolerated {
			if strings.Contains(ln, h) {
				ok = true
				break
			}
		}
		if !ok {
			bad = append(bad, strings.TrimSpace(ln))
		}
	}
	return bad
}

func TestUISmoke(t *testing.T) {
	bin := chromeBin(t)
	offline := os.Getenv("STDOCS_UI_SMOKE_OFFLINE") == "1"
	// Stoplight Elements lazy-renders operation summaries on nav
	// expansion, so its smoke markers are the booted shell: the title,
	// the tag group, and the parsed schema name — which still proves
	// assets loaded, the component ran, and the spec was fetched and
	// parsed. Every other UI renders summaries directly.
	opMarkers := []string{"Get widget by id", "Create widget"}
	stoplightMarkers := []string{"Smoke API", "Widgets", "Widget"}
	// Scalar's CSP-safe defaults must hide the phone-home chrome out of
	// the box (no caller config), so the unconfigured page is clean.
	scalarChromeAbsent := []string{"Ask AI", "Generate MCP", "Developer Tools"}
	uis := []struct {
		name    string
		cdn     bool
		option  stdocs.Option
		markers []string
		absent  []string // must NOT appear in the unconfigured page (default effect)
	}{
		{"builtin", false, nil, opMarkers, nil},
		{"scalaremb", false, scalaremb.WithUI(), opMarkers, scalarChromeAbsent},
		{"swaggeruiemb", false, swaggeruiemb.WithUI(), opMarkers, nil},
		{"redocemb", false, redocemb.WithUI(), opMarkers, nil},
		{"stoplightemb", false, stoplightemb.WithUI(), stoplightMarkers, nil},
		{"scalar", true, scalar.WithUI(), opMarkers, scalarChromeAbsent},
		{"swaggerui", true, swaggerui.WithUI(), opMarkers, nil},
		{"redoc", true, redoc.WithUI(), opMarkers, nil},
		{"stoplight", true, stoplight.WithUI(), stoplightMarkers, nil},
	}
	for _, ui := range uis {
		t.Run(ui.name, func(t *testing.T) {
			if ui.cdn && offline {
				t.Skip("offline run")
			}
			var opts []stdocs.Option
			if ui.option != nil {
				opts = append(opts, ui.option)
			}
			mux := corpusMux(opts...)
			srv := httptest.NewServer(mux)
			defer srv.Close()

			// Embedded UIs: every asset under the docs prefix must
			// resolve — a 404 here is the silent-blank-page bug.
			if strings.HasSuffix(ui.name, "emb") {
				page, err := http.Get(srv.URL + "/docs/")
				if err != nil {
					t.Fatalf("docs page: %v", err)
				}
				page.Body.Close()
				if page.StatusCode != 200 {
					t.Fatalf("docs page status: %d", page.StatusCode)
				}
			}

			allFound := func(dom string) bool {
				for _, m := range ui.markers {
					if !strings.Contains(dom, m) {
						return false
					}
				}
				return true
			}
			deadline := time.Now().Add(60 * time.Second)
			var dom string
			for time.Now().Before(deadline) {
				dom = renderDOM(t, bin, srv.URL+"/smoketest")
				if allFound(dom) {
					break
				}
				time.Sleep(2 * time.Second)
			}
			if !allFound(dom) {
				t.Fatalf("%s did not render its markers %v; DOM %d bytes: %.600s",
					ui.name, ui.markers, len(dom), dom)
			}
			if bad := cspFailures(dom); len(bad) > 0 {
				t.Errorf("%s: %d disallowed CSP violation(s) under its enforced policy; first: %s",
					ui.name, len(bad), bad[0])
			}
			for _, m := range ui.absent {
				if strings.Contains(dom, m) {
					t.Errorf("%s: CSP-safe default should hide %q in the unconfigured page, but it is present", ui.name, m)
				}
			}
		})
	}
}

// TestUISmokeWithConfig renders each rich UI with a representative
// WithConfiguration in a real browser: it must still boot (its markers
// render) and raise no disallowed CSP violation. This is the end-to-end
// gate for the per-UI config carriers — that Scalar honours
// data-configuration, that Swagger UI and Redoc read their JSON carrier
// under the re-pinned/new inline-init hash, and that Stoplight accepts
// attributes — all under each UI's enforced policy.
func TestUISmokeWithConfig(t *testing.T) {
	bin := chromeBin(t)
	offline := os.Getenv("STDOCS_UI_SMOKE_OFFLINE") == "1"
	opMarkers := []string{"Get widget by id", "Create widget"}
	stoplightMarkers := []string{"Smoke API", "Widgets", "Widget"}
	// Configs chosen so their effect is assertable in the DOM: Scalar
	// re-enables the chrome its defaults hide (present, proving overrides
	// win); Swagger's displayOperationId adds visible text (present);
	// Stoplight's hideExport removes visible text (absent). Redoc is
	// boot+CSP only (its config effects are colour/layout, not text).
	scalarReenable := map[string]any{
		"showDeveloperTools": "always",
		"mcp":                map[string]any{"disabled": false},
		"agent":              map[string]any{"disabled": false},
	}
	uis := []struct {
		name    string
		cdn     bool
		option  stdocs.Option
		markers []string
		present []string // config-effect text that must appear
		absent  []string // config-effect text that must be gone
	}{
		{"scalaremb", false, scalaremb.WithUI(scalaremb.WithConfiguration(scalarReenable)), opMarkers, []string{"Ask AI", "Generate MCP"}, nil},
		{"swaggeruiemb", false, swaggeruiemb.WithUI(swaggeruiemb.WithConfiguration(map[string]any{"filter": true, "displayOperationId": true})), opMarkers, []string{"post_widgets"}, nil},
		{"redocemb", false, redocemb.WithUI(redocemb.WithConfiguration(map[string]any{"hideDownloadButton": true})), opMarkers, nil, nil},
		{"stoplightemb", false, stoplightemb.WithUI(stoplightemb.WithConfiguration(map[string]any{"hideExport": true, "hideTryItPanel": true})), stoplightMarkers, nil, []string{"Export"}},
		{"scalar", true, scalar.WithUI(scalar.WithConfiguration(scalarReenable)), opMarkers, []string{"Ask AI", "Generate MCP"}, nil},
		{"swaggerui", true, swaggerui.WithUI(swaggerui.WithConfiguration(map[string]any{"filter": true, "displayOperationId": true})), opMarkers, []string{"post_widgets"}, nil},
		{"redoc", true, redoc.WithUI(redoc.WithConfiguration(map[string]any{"hideDownloadButton": true})), opMarkers, nil, nil},
		{"stoplight", true, stoplight.WithUI(stoplight.WithConfiguration(map[string]any{"hideExport": true, "hideTryItPanel": true})), stoplightMarkers, nil, []string{"Export"}},
	}
	for _, ui := range uis {
		t.Run(ui.name, func(t *testing.T) {
			if ui.cdn && offline {
				t.Skip("offline run")
			}
			srv := httptest.NewServer(corpusMux(ui.option))
			defer srv.Close()
			allFound := func(dom string) bool {
				for _, m := range ui.markers {
					if !strings.Contains(dom, m) {
						return false
					}
				}
				return true
			}
			deadline := time.Now().Add(60 * time.Second)
			var dom string
			for time.Now().Before(deadline) {
				dom = renderDOM(t, bin, srv.URL+"/smoketest")
				if allFound(dom) {
					break
				}
				time.Sleep(2 * time.Second)
			}
			if !allFound(dom) {
				t.Fatalf("%s did not render its markers %v with config; DOM %d bytes: %.600s",
					ui.name, ui.markers, len(dom), dom)
			}
			if bad := cspFailures(dom); len(bad) > 0 {
				t.Errorf("%s with config: %d disallowed CSP violation(s); first: %s",
					ui.name, len(bad), bad[0])
			}
			for _, m := range ui.present {
				if !strings.Contains(dom, m) {
					t.Errorf("%s with config: expected effect %q to be present, but it is not", ui.name, m)
				}
			}
			for _, m := range ui.absent {
				if strings.Contains(dom, m) {
					t.Errorf("%s with config: expected config to hide %q, but it is still present", ui.name, m)
				}
			}
		})
	}
}

// TestDefaultUINoticeDismiss drives the built-in page's dismissable
// notice in a real browser, under its enforced CSP: a same-origin
// driver frames /docs/, clicks the dismiss button, and reports whether
// the notice hid and the preference landed in localStorage.
func TestDefaultUINoticeDismiss(t *testing.T) {
	bin := chromeBin(t)
	const driver = `<!doctype html><html><body><div id="out">PENDING</div>
<iframe id="f" src="/docs/" style="width:900px;height:500px"></iframe>
<script>
var f=document.getElementById('f'),n=0;
var timer=setInterval(function(){
  n++;
  try{
    var doc=f.contentDocument, win=f.contentWindow;
    var note=doc.getElementById('stdocs-note');
    if(note && note.hidden===false){
      doc.getElementById('stdocs-note-x').click();
      var hiddenAfter=doc.getElementById('stdocs-note').hidden;
      var ls=win.localStorage.getItem('stdocs-docs-notice-dismissed');
      document.getElementById('out').textContent='RESULT hiddenAfterClick='+hiddenAfter+' localStorage='+ls;
      clearInterval(timer);
    }
  }catch(e){}
  if(n>60){document.getElementById('out').textContent='TIMEOUT';clearInterval(timer);}
},250);
</script></body></html>`
	mux := stdocs.New(stdocs.WithTitle("Notice"))
	mux.HandleFunc("GET /x", func(w http.ResponseWriter, r *http.Request) {}, stdocs.Summary("x"))
	mux.HandleFunc("GET /driver", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(driver))
	}, stdocs.Hidden())
	mux.Mount()
	srv := httptest.NewServer(mux)
	defer srv.Close()

	dom := renderDOM(t, bin, srv.URL+"/driver")
	if !strings.Contains(dom, "RESULT hiddenAfterClick=true localStorage=1") {
		t.Fatalf("notice dismiss flow failed; DOM %d bytes: %.400s", len(dom), dom)
	}
}

// TestDefaultUIExpand drives the built-in page's accordion in a real
// browser, under its enforced CSP: a same-origin driver frames /docs/,
// clicks the "Create widget" operation, and checks a detail panel with a
// JSON example appears.
func TestDefaultUIExpand(t *testing.T) {
	bin := chromeBin(t)
	const driver = `<!doctype html><html><body><div id="out">PENDING</div>
<iframe id="f" src="/docs/" style="width:1000px;height:700px"></iframe>
<script>
var f=document.getElementById('f'),n=0;
var timer=setInterval(function(){
  n++;
  try{
    var doc=f.contentDocument;
    var rows=doc.querySelectorAll('.op.clickable');
    if(rows.length){
      var target=null;
      rows.forEach(function(r){if(r.textContent.indexOf('Create widget')>=0)target=r;});
      (target||rows[0]).click();
      var detail=doc.querySelector('.detail');
      var pre=doc.querySelector('.detail pre');
      var hasJSON=!!(pre&&pre.textContent.indexOf('{')>=0);
      document.getElementById('out').textContent='RESULT detail='+!!detail+' pre='+!!pre+' json='+hasJSON;
      clearInterval(timer);
    }
  }catch(e){}
  if(n>60){document.getElementById('out').textContent='TIMEOUT';clearInterval(timer);}
},250);
</script></body></html>`
	mux := stdocs.New(stdocs.WithTitle("Expand"))
	mux.HandleFunc("POST /widgets", func(w http.ResponseWriter, r *http.Request) {},
		stdocs.Summary("Create widget"), stdocs.WithBody(Widget{}), stdocs.WithResponse(201, Widget{}))
	mux.HandleFunc("GET /widgets/{id}", func(w http.ResponseWriter, r *http.Request) {},
		stdocs.Summary("Get widget by id"), stdocs.WithResponse(200, Widget{}))
	mux.HandleFunc("GET /expanddriver", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(driver))
	}, stdocs.Hidden())
	mux.Mount()
	srv := httptest.NewServer(mux)
	defer srv.Close()

	dom := renderDOM(t, bin, srv.URL+"/expanddriver")
	if !strings.Contains(dom, "RESULT detail=true pre=true json=true") {
		t.Fatalf("accordion expand failed; DOM %d bytes: %.400s", len(dom), dom)
	}
}
