package redoc

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestPinnedVersion prevents accidental un-pinning of the Redoc
// CDN version. Bumping the version requires also updating the
// SRI hash in redoc.go.
func TestPinnedVersion(t *testing.T) {
	if redocVersion != "2.5.3" {
		t.Errorf("redocVersion = %q, want 2.5.3 (re-run SRI hash update)", redocVersion)
	}
	wantURL := fmt.Sprintf("https://cdn.jsdelivr.net/npm/redoc@%s/bundles/redoc.standalone.js", redocVersion)
	if !strings.Contains(redocHTML, `src="`+wantURL+`"`) {
		t.Errorf("redocHTML must reference the pinned URL %s", wantURL)
	}
	if !strings.Contains(redocHTML, `integrity="`+redocSRIHash+`"`) {
		t.Errorf("redocHTML is missing the integrity hash %s", redocSRIHash)
	}
	if !strings.Contains(redocHTML, `crossorigin="anonymous"`) {
		t.Errorf(`redocHTML must set crossorigin="anonymous" alongside integrity`)
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
	if got := manifest.DevDependencies["redoc"]; got != redocVersion {
		t.Errorf("package.json pins redoc %q but ui/redoc pins %q; a Dependabot bump requires updating the Go version constant, the SRI hashes, and the vendored bundle in ui/redocemb", got, redocVersion)
	}
}
