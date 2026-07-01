package stdocs

import "net/http"

// defaultDocsCSP is the Content-Security-Policy for the built-in docs
// page. That page is self-contained: one inline <script> that only
// builds DOM nodes with textContent and fetches the spec, and one
// inline <style>. Both are pinned by sha256 hash, there are no external
// sources, and there is no 'unsafe-inline'. The hashes are recomputed
// from the actually-served page by TestDefaultDocsCSP, so they cannot
// drift from the HTML without the test failing.
const defaultDocsCSP = "default-src 'none'; base-uri 'none'; form-action 'none'; " +
	"frame-ancestors 'self'; connect-src 'self'; " +
	"script-src 'sha256-hkb2WAiMIQEpsqqZAM+FnfWltUN2gDZ9EEOrXIqVilc='; " +
	"style-src 'sha256-/SFcV+ex7HY6oZdNT1/wTczfG1nny/Vemyk5AjoKUMg='"

// docsPermissionsPolicy denies every powerful browser feature on the
// docs page; an API reference needs none of them. Unrecognised feature
// tokens are ignored by the browser, so listing extras is harmless.
const docsPermissionsPolicy = "accelerometer=(), autoplay=(), camera=(), " +
	"display-capture=(), encrypted-media=(), fullscreen=(), geolocation=(), " +
	"gyroscope=(), magnetometer=(), microphone=(), midi=(), payment=(), " +
	"picture-in-picture=(), publickey-credentials-get=(), screen-wake-lock=(), " +
	"sync-xhr=(), usb=(), web-share=(), xr-spatial-tracking=()"

// setDocsBaselineHeaders writes the hardening headers shared by every
// docs response, HTML and spec alike. The Content-Security-Policy is
// page-specific and added by the caller for the HTML response only; it
// does nothing for a JSON or YAML body.
func setDocsBaselineHeaders(h http.Header) {
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("Referrer-Policy", "no-referrer")
	h.Set("X-Frame-Options", "SAMEORIGIN")
	h.Set("Permissions-Policy", docsPermissionsPolicy)
}
