package redocemb

import (
	"crypto/sha512"
	"encoding/base64"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

// redocJSHash is the sha384 hash of the vendored
// assets/redoc.standalone.js, verified byte-identical to
// bundles/redoc.standalone.js in the redoc@2.5.3 npm tarball (and
// to the pinned jsDelivr URL). It matches redocSRIHash in ui/redoc.
const redocJSHash = "sha384-xiEssMQFSpSfLbzRZCGfxxIM5QDb2DTrU6vyoZdp2sV1L6pmOMy6MpTtUoLbpC96"

func sri384(data []byte) string {
	sum := sha512.Sum384(data)
	return "sha384-" + base64.StdEncoding.EncodeToString(sum[:])
}

// TestEmbeddedAssetIntegrity pins the vendored bundle bytes to the
// published npm release. A missing or tampered asset is a hard
// failure, never a skip: the module ships the bundle in-repo.
func TestEmbeddedAssetIntegrity(t *testing.T) {
	data, err := fs.ReadFile(assetsSubFS, "redoc.standalone.js")
	if err != nil {
		t.Fatalf("embedded redoc.standalone.js missing: %v (the bundle must be vendored in assets/)", err)
	}
	if got := sri384(data); got != redocJSHash {
		t.Fatalf("embedded redoc.standalone.js hash = %s, want %s (re-vendor from redoc@%s and update the pins)", got, redocJSHash, redocVersion)
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
	if got := manifest.DevDependencies["redoc"]; got != redocVersion {
		t.Errorf("package.json pins redoc %q but ui/redocemb pins %q; a Dependabot bump requires updating the Go version constant, the SRI hashes, and the vendored bundle", got, redocVersion)
	}
}
