package stdocs

import (
	"github.com/FumingPower3925/stdocs/internal/spec"
	"github.com/FumingPower3925/stdocs/internal/version"
)

// Public re-exports of spec types. These are type aliases so values
// are interchangeable between stdocs.X and internal/spec.X.
type (
	Info        = spec.Info
	Contact     = spec.Contact
	License     = spec.License
	Server      = spec.Server
	TagDecl     = spec.TagDecl
	Param       = spec.Param
	Response    = spec.Response
	RequestBody = spec.RequestBody
	Operation   = spec.Operation
	PathItem    = spec.PathItem
	SpecInput   = spec.SpecInput
)

// SpecVersion is the OpenAPI spec version (3.0.4, 3.1.2, or 3.2.0).
type SpecVersion = version.SpecVersion

// OpenAPI30, OpenAPI31, and OpenAPI32 are the version constants for
// the supported OpenAPI spec versions (3.0.4, 3.1.2, 3.2.0
// respectively). stdocs only ships the latest patch of each minor.
const (
	OpenAPI30 = version.OpenAPI30
	OpenAPI31 = version.OpenAPI31
	OpenAPI32 = version.OpenAPI32
)
