package swaggerui

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestPinnedVersion prevents accidental un-pinning of the Swagger
// UI CDN version. Bumping the version requires also updating the
// SRI hashes in swaggerui.go.
func TestPinnedVersion(t *testing.T) {
	if swaggerUIVersion != "5.32.6" {
		t.Errorf("swaggerUIVersion = %q, want 5.32.6 (re-run SRI hash update)", swaggerUIVersion)
	}
	wantJS := fmt.Sprintf("https://cdn.jsdelivr.net/npm/swagger-ui-dist@%s/swagger-ui-bundle.js", swaggerUIVersion)
	if !strings.Contains(swaggerHTML, `src="`+wantJS+`"`) {
		t.Errorf("swaggerHTML must reference the pinned URL %s", wantJS)
	}
	wantCSS := fmt.Sprintf("https://cdn.jsdelivr.net/npm/swagger-ui-dist@%s/swagger-ui.css", swaggerUIVersion)
	if !strings.Contains(swaggerHTML, `href="`+wantCSS+`"`) {
		t.Errorf("swaggerHTML must link the pinned stylesheet %s", wantCSS)
	}
	if !strings.Contains(swaggerHTML, `integrity="`+swaggerUIBundleHash+`"`) {
		t.Errorf("swaggerHTML is missing the bundle integrity hash %s", swaggerUIBundleHash)
	}
	if !strings.Contains(swaggerHTML, `integrity="`+swaggerUICSSHash+`"`) {
		t.Errorf("swaggerHTML is missing the CSS integrity hash %s", swaggerUICSSHash)
	}
	if got := strings.Count(swaggerHTML, `crossorigin="anonymous"`); got < 2 {
		t.Errorf(`swaggerHTML must set crossorigin="anonymous" on both SRI tags, found %d`, got)
	}
	// SwaggerUIStandalonePreset ships in swagger-ui-standalone-preset.js,
	// which this page never loads; referencing it injects `undefined`
	// into the presets list.
	if strings.Contains(swaggerHTML, "SwaggerUIStandalonePreset") {
		t.Errorf("swaggerHTML must not reference SwaggerUIStandalonePreset (not part of swagger-ui-bundle.js)")
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
	if got := manifest.DevDependencies["swagger-ui-dist"]; got != swaggerUIVersion {
		t.Errorf("package.json pins swagger-ui-dist %q but ui/swaggerui pins %q; a Dependabot bump requires updating the Go version constant, the SRI hashes, and the vendored bundle in ui/swaggeruiemb", got, swaggerUIVersion)
	}
}
