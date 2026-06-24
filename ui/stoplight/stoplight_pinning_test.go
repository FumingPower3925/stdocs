package stoplight

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestPinnedVersion prevents accidental un-pinning of the
// Stoplight Elements CDN version. Both CDN URLs point at verbatim
// files from the npm tarball, so their sha384 SRI hashes are
// pinned too.
func TestPinnedVersion(t *testing.T) {
	if stoplightVersion != "9.0.23" {
		t.Errorf("stoplightVersion = %q, want 9.0.23", stoplightVersion)
	}
	wantJS := fmt.Sprintf("https://cdn.jsdelivr.net/npm/@stoplight/elements@%s/web-components.min.js", stoplightVersion)
	if !strings.Contains(stoplightHTML, `src="`+wantJS+`"`) {
		t.Errorf("stoplightHTML must reference the pinned URL %s", wantJS)
	}
	wantCSS := fmt.Sprintf("https://cdn.jsdelivr.net/npm/@stoplight/elements@%s/styles.min.css", stoplightVersion)
	if !strings.Contains(stoplightHTML, `href="`+wantCSS+`"`) {
		t.Errorf("stoplightHTML must link the pinned stylesheet %s (Elements renders unstyled without it)", wantCSS)
	}
	if !strings.Contains(stoplightHTML, `integrity="`+stoplightJSHash+`"`) {
		t.Errorf("stoplightHTML is missing the script integrity hash %s", stoplightJSHash)
	}
	if !strings.Contains(stoplightHTML, `integrity="`+stoplightCSSHash+`"`) {
		t.Errorf("stoplightHTML is missing the stylesheet integrity hash %s", stoplightCSSHash)
	}
	if got := strings.Count(stoplightHTML, `crossorigin="anonymous"`); got < 2 {
		t.Errorf(`stoplightHTML must set crossorigin="anonymous" on both SRI tags, found %d`, got)
	}
}

// TestNPMManifestParity keeps the Go pin in lockstep with the
// repo-root package.json that Dependabot watches.
func TestNPMManifestParity(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "package.json"))
	if err != nil {
		t.Fatalf("read package.json: %v", err)
	}
	var manifest struct {
		DevDependencies map[string]string `json:"devDependencies"`
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatalf("parse package.json: %v", err)
	}
	if got := manifest.DevDependencies["@stoplight/elements"]; got != stoplightVersion {
		t.Errorf("package.json pins @stoplight/elements %q but ui/stoplight pins %q; a Dependabot bump requires updating the Go version constant, the SRI hashes, and the vendored bundle in ui/stoplightemb", got, stoplightVersion)
	}
}
