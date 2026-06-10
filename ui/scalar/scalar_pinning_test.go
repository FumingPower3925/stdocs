package scalar

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestPinnedVersion prevents accidental un-pinning of the Scalar
// CDN version. The script URL points at the verbatim
// dist/browser/standalone.js file, so its sha384 SRI hash is
// pinned too.
func TestPinnedVersion(t *testing.T) {
	if scalarVersion != "1.59.2" {
		t.Errorf("scalarVersion = %q, want 1.59.2 (re-vendor the bundle in ui/scalaremb)", scalarVersion)
	}
	wantURL := fmt.Sprintf("https://cdn.jsdelivr.net/npm/@scalar/api-reference@%s/dist/browser/standalone.js", scalarVersion)
	if !strings.Contains(scalarHTML, `src="`+wantURL+`"`) {
		t.Errorf("scalarHTML must reference the pinned URL %s", wantURL)
	}
	if !strings.Contains(scalarHTML, `integrity="`+scalarSRIHash+`"`) {
		t.Errorf("scalarHTML is missing the integrity hash %s", scalarSRIHash)
	}
	if !strings.Contains(scalarHTML, `crossorigin="anonymous"`) {
		t.Errorf(`scalarHTML must set crossorigin="anonymous" alongside integrity`)
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
	if got := manifest.DevDependencies["@scalar/api-reference"]; got != scalarVersion {
		t.Errorf("package.json pins @scalar/api-reference %q but ui/scalar pins %q; a Dependabot bump requires updating the Go version constant, the SRI hashes, and the vendored bundle in ui/scalaremb", got, scalarVersion)
	}
}
