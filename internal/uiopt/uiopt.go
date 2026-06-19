// Package uiopt holds the option machinery shared by the bundled UI
// sub-packages (ui/scalar, ui/swaggerui, ui/redoc, ui/stoplight and
// their embedded twins). It is internal: only stdocs's own UI packages
// import it. Each UI package re-exports Option as its own UIOption type
// and Configuration as its own WithConfiguration, so the wiring lives in
// one place while the public API stays per-package.
package uiopt

// Settings accumulates the options passed to a UI sub-package's WithUI.
type Settings struct {
	// Config is the UI-native configuration set by WithConfiguration,
	// forwarded to stdocs.Config.UIConfig. nil when unset.
	Config map[string]any
}

// Option mutates Settings.
type Option func(*Settings)

// Apply folds opts into a fresh Settings.
func Apply(opts []Option) Settings {
	var s Settings
	for _, o := range opts {
		o(&s)
	}
	return s
}

// Configuration is the shared implementation of every UI's
// WithConfiguration: it records the configuration map for the UI to
// render into its docs page.
func Configuration(cfg map[string]any) Option {
	return func(s *Settings) { s.Config = cfg }
}

// Merge overlays over onto a copy of base, with over winning on key
// conflicts (a shallow, top-level merge). UI sub-packages use it to apply
// their CSP-safe defaults while letting a caller's WithConfiguration
// override any key. It returns nil when the result is empty, so a UI with
// no defaults and no caller config renders no configuration carrier and
// stays byte-identical to the unconfigured page.
func Merge(base, over map[string]any) map[string]any {
	if len(base) == 0 && len(over) == 0 {
		return nil
	}
	out := make(map[string]any, len(base)+len(over))
	for k, v := range base {
		out[k] = v
	}
	for k, v := range over {
		out[k] = v
	}
	return out
}
