package stoplightemb

import (
	"crypto/sha512"
	"encoding/base64"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

// sha384 hashes of the vendored assets, verified byte-identical to
// the files in the @stoplight/elements@9.0.24 npm tarball (and to
// the pinned jsDelivr URLs). They match the SRI hashes in
// ui/stoplight.
const (
	webComponentsJSHash = "sha384-Kx8v0VsAmmNDqBDAOnY3pQFLUNZNwhakX114rKqExXeXBbDgXHBvasXBU8QxWSMB"
	stylesCSSHash       = "sha384-iVQBHadsD+eV0M5+ubRCEVXrXEBj+BqcuwjUwPoVJc0Pb1fmrhYSAhL+BFProHdV"
)

func sri384(data []byte) string {
	sum := sha512.Sum384(data)
	return "sha384-" + base64.StdEncoding.EncodeToString(sum[:])
}

// TestEmbeddedAssetIntegrity pins the vendored bundle bytes to the
// published npm release. A missing or tampered asset is a hard
// failure, never a skip: the module ships the bundle in-repo.
func TestEmbeddedAssetIntegrity(t *testing.T) {
	for name, want := range map[string]string{
		"web-components.min.js": webComponentsJSHash,
		"styles.min.css":        stylesCSSHash,
	} {
		data, err := fs.ReadFile(assetsSubFS, name)
		if err != nil {
			t.Fatalf("embedded %s missing: %v (the bundle must be vendored in assets/)", name, err)
		}
		if got := sri384(data); got != want {
			t.Fatalf("embedded %s hash = %s, want %s (re-vendor from @stoplight/elements@%s and update the pins)", name, got, want, stoplightVersion)
		}
	}
}

// TestNPMManifestParity keeps the vendored pin in lockstep with the
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
		t.Errorf("package.json pins @stoplight/elements %q but ui/stoplightemb pins %q; a Dependabot bump requires updating the Go version constant, the SRI hashes, and the vendored bundle", got, stoplightVersion)
	}
}
