// Package tsbridge hands the version-agnostic document model from the
// root package to the tsgen subpackage. The root package assigns
// Generate at init; tsgen calls it. The indirection exists so the
// model never becomes public API surface — outside this module the
// variable is unreachable.
package tsbridge

import "github.com/FumingPower3925/stdocs/internal/spec"

// Generate builds the document model for a *stdocs.Mux (typed any to
// avoid an import cycle) and runs consume on it UNDER THE BUILD LOCK
// — the model holds the registry's live operation pointers, which
// concurrent builds mutate in place, so reading them outside the
// lock races. It shares Mux.JSON's fail-fast surface: tag panics and
// validation errors fire here the same way.
var Generate func(mux any, consume func(spec.SpecInput) []byte) ([]byte, error)
