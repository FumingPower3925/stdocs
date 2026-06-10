package redoc

import (
	"strings"
	"testing"
)

// TestPinnedVersion prevents accidental un-pinning of the Redoc
// CDN version. Bumping the version requires also updating the
// SRI hash in redoc.go and the test in this file.
func TestPinnedVersion(t *testing.T) {
	if redocVersion != "2.5.3" {
		t.Errorf("redocVersion = %q, want 2.5.3 (re-run SRI hash update)", redocVersion)
	}
	if !strings.Contains(redocHTML, "redoc@2.5.3/bundles/redoc.standalone.js") {
		t.Errorf("redocHTML must reference the pinned version 2.5.3")
	}
	if !strings.Contains(redocHTML, redocSRIHash) {
		t.Errorf("redocHTML is missing the integrity hash %s", redocSRIHash)
	}
}
