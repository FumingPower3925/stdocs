// Package tsbridge hands the version-agnostic document model from the
// root package to the tsgen subpackage. The root package assigns
// SpecInput at init; tsgen calls it. The indirection exists so the
// model never becomes public API surface — outside this module the
// variable is unreachable.
package tsbridge

import "github.com/FumingPower3925/stdocs/internal/spec"

// SpecInput builds and returns the document model for a *stdocs.Mux
// (typed any to avoid an import cycle). It shares Mux.JSON's
// fail-fast surface: tag panics and validation errors fire here the
// same way.
var SpecInput func(mux any) (spec.SpecInput, error)
