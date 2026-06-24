package swaggeruiemb

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
// the files in the swagger-ui-dist@5.32.8 npm tarball (and to the
// pinned jsDelivr URLs). They match the SRI hashes in ui/swaggerui.
const (
	bundleJSHash = "sha384-IKpAWwsTL0pcw7/Amtnt2eXF4P1BK64WNuY2E/RG15SWLUW5HXzFuyqCSAr/DP8C"
	cssHash      = "sha384-9Q2fpS+xeS4ffJy6CagnwoUl+4ldAYhOs9pgZuEKxypVModhmZFzeMlvVsAjf7uT"
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
		"swagger-ui-bundle.js": bundleJSHash,
		"swagger-ui.css":       cssHash,
	} {
		data, err := fs.ReadFile(assetsSubFS, name)
		if err != nil {
			t.Fatalf("embedded %s missing: %v (the bundle must be vendored in assets/)", name, err)
		}
		if got := sri384(data); got != want {
			t.Fatalf("embedded %s hash = %s, want %s (re-vendor from swagger-ui-dist@%s and update the pins)", name, got, want, swaggerUIVersion)
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
	if got := manifest.DevDependencies["swagger-ui-dist"]; got != swaggerUIVersion {
		t.Errorf("package.json pins swagger-ui-dist %q but ui/swaggeruiemb pins %q; a Dependabot bump requires updating the Go version constant, the SRI hashes, and the vendored bundle", got, swaggerUIVersion)
	}
}
