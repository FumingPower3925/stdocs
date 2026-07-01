package scalaremb

import (
	"crypto/sha512"
	"encoding/base64"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

// standaloneJSHash is the sha384 hash of the vendored
// assets/standalone.js, verified byte-identical to
// dist/browser/standalone.js in the @scalar/api-reference@1.62.1
// npm tarball (and to the pinned jsDelivr URL). It matches
// scalarSRIHash in ui/scalar.
const standaloneJSHash = "sha384-nwhiadu/j7QCPGQnDNV889i24StuTIjh9zCxj+J0rr33/d50AWFHImGdxArEi8IB"

func sri384(data []byte) string {
	sum := sha512.Sum384(data)
	return "sha384-" + base64.StdEncoding.EncodeToString(sum[:])
}

// TestEmbeddedAssetIntegrity pins the vendored bundle bytes to the
// published npm release. A missing or tampered asset is a hard
// failure, never a skip: the module ships the bundle in-repo.
func TestEmbeddedAssetIntegrity(t *testing.T) {
	data, err := fs.ReadFile(assetsSubFS, "standalone.js")
	if err != nil {
		t.Fatalf("embedded standalone.js missing: %v (the bundle must be vendored in assets/)", err)
	}
	if got := sri384(data); got != standaloneJSHash {
		t.Fatalf("embedded standalone.js hash = %s, want %s (re-vendor from @scalar/api-reference@%s and update the pins)", got, standaloneJSHash, scalarVersion)
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
	if got := manifest.DevDependencies["@scalar/api-reference"]; got != scalarVersion {
		t.Errorf("package.json pins @scalar/api-reference %q but ui/scalaremb pins %q; a Dependabot bump requires updating the Go version constant, the SRI hashes, and the vendored bundle", got, scalarVersion)
	}
}
