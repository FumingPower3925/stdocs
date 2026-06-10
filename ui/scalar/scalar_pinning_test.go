package scalar

import (
	"strings"
	"testing"
)

// TestPinnedVersion prevents accidental un-pinning of the Scalar
// CDN version. SRI is not pinned for the CDN sub-package because
// jsDelivr generates the bundle on the fly; for SRI use the
// ui/scalaremb sub-package instead.
func TestPinnedVersion(t *testing.T) {
	if scalarVersion != "1.59.2" {
		t.Errorf("scalarVersion = %q, want 1.59.2 (re-vendor the bundle in ui/scalaremb)", scalarVersion)
	}
	if !strings.Contains(scalarHTML, "@scalar/api-reference@1.59.2") {
		t.Errorf("scalarHTML must reference the pinned version 1.59.2")
	}
}
