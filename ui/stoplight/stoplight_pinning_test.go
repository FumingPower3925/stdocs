package stoplight

import (
	"strings"
	"testing"
)

// TestPinnedVersion prevents accidental un-pinning of the
// Stoplight Elements CDN version. SRI is not pinned because
// jsDelivr generates the bundle on the fly.
func TestPinnedVersion(t *testing.T) {
	if stoplightVersion != "9.0.22" {
		t.Errorf("stoplightVersion = %q, want 9.0.22", stoplightVersion)
	}
	if !strings.Contains(stoplightHTML, "@stoplight/elements@9.0.22/web-components.min.js") {
		t.Errorf("stoplightHTML must reference the pinned version 9.0.22")
	}
}
