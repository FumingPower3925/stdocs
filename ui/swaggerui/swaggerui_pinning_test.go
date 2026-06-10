package swaggerui

import (
	"strings"
	"testing"
)

// TestPinnedVersion prevents accidental un-pinning of the Swagger
// UI CDN version. Bumping the version requires also updating the
// SRI hashes in swaggerui.go and the test in this file.
func TestPinnedVersion(t *testing.T) {
	if swaggerUIVersion != "5.32.6" {
		t.Errorf("swaggerUIVersion = %q, want 5.32.6 (re-run SRI hash update)", swaggerUIVersion)
	}
	if !strings.Contains(swaggerHTML, "swagger-ui-dist@5.32.6/swagger-ui-bundle.js") {
		t.Errorf("swaggerHTML must reference the pinned version 5.32.6")
	}
	if !strings.Contains(swaggerHTML, "swagger-ui-dist@5.32.6/swagger-ui.css") {
		t.Errorf("swaggerHTML must reference the pinned version 5.32.6")
	}
	if !strings.Contains(swaggerHTML, swaggerUIBundleHash) {
		t.Errorf("swaggerHTML is missing the bundle integrity hash %s", swaggerUIBundleHash)
	}
	if !strings.Contains(swaggerHTML, swaggerUICSSHash) {
		t.Errorf("swaggerHTML is missing the CSS integrity hash %s", swaggerUICSSHash)
	}
}
