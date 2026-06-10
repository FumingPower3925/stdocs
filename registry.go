package stdocs

import (
	"sort"
	"strings"

	"github.com/FumingPower3925/stdocs/internal/pattern"
	"github.com/FumingPower3925/stdocs/internal/schema"
)

// route is the internal record held by the registry for each registered
// pattern. It carries the original pattern, the parsed Pattern, the
// partial Operation under construction, the handler's function name
// (for default summary inference), and the version of the parent mux.
type route struct {
	// pattern is the original pattern string as passed to HandleFunc.
	pattern string
	// parsed is the parsed Pattern, computed once at registration.
	parsed *pattern.Pattern
	// funcName is the name of the handler function, for inference.
	funcName string
	// op is the operation under construction. RouteOpts mutate this.
	op *Operation
	// version is inherited from the parent mux config.
	version SpecVersion
}

// registry is the collection of routes. It is safe to read concurrently
// after all routes have been added; the spec emitter does the read.
type registry struct {
	routes []*route
}

// add registers a new route. opts are applied to construct the operation.
// version is the OpenAPI version inherited from the parent mux.
func (r *registry) add(pattern, funcName string, parsed *pattern.Pattern, version SpecVersion, opts []RouteOpt) *route {
	rt := &route{
		pattern:  pattern,
		parsed:   parsed,
		funcName: funcName,
		op:       &Operation{},
		version:  version,
	}
	for _, o := range opts {
		o(rt)
	}
	r.routes = append(r.routes, rt)
	return rt
}

// finalize applies the per-route defaults that require knowing the
// config: auto-200, default tag from first path segment, default summary
// from function name, and operationId from method+path.
func (r *registry) finalize(cfg *Config) {
	for _, rt := range r.routes {
		// Auto-200: if no responses were declared, add a default 200.
		if len(rt.op.Responses) == 0 {
			rt.op.Responses = map[string]*Response{
				"200": {Status: "200", Description: "OK"},
			}
		}

		// Method from pattern.
		if rt.op.Method == "" {
			rt.op.Method = rt.parsed.Method
		}
		// Custom HTTP methods (PURGE, etc.) are valid in stdlib
		// ServeMux but are not legal as keys of an OpenAPI Path Item
		// Object. We accept the registration (the user is going to
		// need the route to actually work) but record a warning
		// in the spec via an x-stdocs-warning extension on the
		// operation, and the operation key the emitter produces
		// is still the lowercased method. Strict validators
		// (Spectral, openapi-spec-validator) will flag this; the
		// extension makes the cause visible.
		if rt.parsed.Method != "" && !pattern.IsOpenAPIMethod(rt.parsed.Method) {
			if rt.op.Extensions == nil {
				rt.op.Extensions = map[string]any{}
			}
			rt.op.Extensions["x-stdocs-warning"] = "method " + rt.parsed.Method +
				" is not a legal OpenAPI Path Item key; the spec may fail strict validation"
		}

		// Path-level parameters (shared across methods of the same path)
		// come from wildcards in the pattern. We add them only at the
		// operation level here; if the user registers multiple methods
		// on the same path, the emitter will see them all.
		//
		// For now, append path parameters that are not already in the
		// operation's parameter list. (The spec emitter handles dedup.)
		existingNames := make(map[string]bool)
		for _, p := range rt.op.Parameters {
			existingNames[p.Name] = true
		}
		for _, name := range rt.parsed.WildcardNames() {
			if name == "" {
				// Defensive: anonymous wildcards (e.g. the implicit
				// multi from "/") should not be emitted as parameters.
				// WildcardNames already filters these, but be
				// defensive at the use site too.
				continue
			}
			if existingNames[name] {
				continue
			}
			rt.op.Parameters = append(rt.op.Parameters, Param{
				Name:        name,
				In:          "path",
				Required:    true,
				Description: "",
				Schema:      &schema.Schema{Type: "string"},
			})
		}

		// Default summary: from function name, or from DefaultSummary
		// template, but only if Summary was not provided.
		if rt.op.Summary == "" {
			if s := summaryFromFuncName(rt.funcName); s != "" {
				rt.op.Summary = s
			} else if cfg.DefaultSummary != "" {
				rt.op.Summary = cfg.DefaultSummary
			}
		}

		// Default tag: from first path segment, but only if no tags
		// were provided.
		if len(rt.op.Tags) == 0 {
			if t := tagFromPath(rt.pattern); t != "" {
				rt.op.Tags = []string{t}
			}
		}

		// Default operationId from method+path.
		if rt.op.OperationID == "" {
			rt.op.OperationID = defaultOperationID(rt.parsed)
		}
	}

	// Operation-IDs must be document-wide unique. Disambiguate
	// collisions with a numeric suffix. The first occurrence keeps
	// its name; subsequent collisions become "id", "id_2", "id_3"...
	seen := make(map[string]int)
	for _, rt := range r.routes {
		id := rt.op.OperationID
		if n, exists := seen[id]; exists {
			rt.op.OperationID = id + "_" + itoa(n+1)
			seen[id] = n + 1
		} else {
			seen[id] = 1
		}
	}
}

// itoa is a small allocation-free int-to-string converter used by
// the operation-Id disambiguator. It avoids pulling in strconv for
// the same reason schema/Schema doesn't.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}

// defaultOperationID builds an operationId like "get_users_by_id"
// from a parsed pattern. The method is lower-cased; path segments are
// joined by underscores; wildcards are prefixed with "by_" to
// distinguish them from same-named literals (e.g. "GET /users/{id}"
// and "GET /users/id" produce different ids: "get_users_by_id" and
// "get_users_id"). When the wildcard is the trailing multi, we
// append "rest" to mark the rest-of-path semantics.
func defaultOperationID(p *pattern.Pattern) string {
	method := strings.ToLower(p.Method)
	if method == "" {
		method = "any"
	}
	parts := []string{method}
	for _, s := range p.Segments {
		var v string
		switch s.Kind {
		case pattern.KindLiteral:
			v = s.Value
		case pattern.KindWildcard:
			v = "by_" + s.Value
		case pattern.KindMulti:
			if s.Value == "" {
				// Anonymous trailing multi from "/posts/".
				v = "rest"
			} else {
				v = "by_" + s.Value + "_rest"
			}
		case pattern.KindTrailing:
			v = "root"
		}
		if v != "" {
			parts = append(parts, v)
		}
	}
	out := ""
	for i, p := range parts {
		if i > 0 {
			out += "_"
		}
		out += p
	}
	return out
}

// toPathItems flattens the routes into PathItems, grouped by path. The
// per-path Parameter slice is built from the union of path-level wildcards
// across all methods of that path; operation-level Parameter slices are
// left alone (the emitter renders them under each method).
func (r *registry) toPathItems() []PathItem {
	// Group by path.
	byPath := make(map[string]*PathItem)
	pathOrder := []string{}
	for _, rt := range r.routes {
		pi, ok := byPath[rt.parsed.Path()]
		if !ok {
			pi = &PathItem{
				Path:       rt.parsed.Path(),
				Operations: make(map[string]*Operation),
			}
			byPath[rt.parsed.Path()] = pi
			pathOrder = append(pathOrder, rt.parsed.Path())
		}
		method := rt.op.Method
		if method == "" {
			method = "GET" // fall back: if user did Handle("pattern", h) with no method, the pattern has no method
		}
		pi.Operations[strings.ToLower(method)] = rt.op
	}
	// Build path-level parameters: union of wildcard names.
	for _, p := range pathOrder {
		pi := byPath[p]
		wildNames := make(map[string]bool)
		for _, rt := range r.routes {
			if rt.parsed.Path() != p {
				continue
			}
			for _, n := range rt.parsed.WildcardNames() {
				wildNames[n] = true
			}
		}
		for n := range wildNames {
			if n == "" {
				// Defensive: skip anonymous wildcards at path level too.
				continue
			}
			pi.Parameters = append(pi.Parameters, Param{
				Name:     n,
				In:       "path",
				Required: true,
				Schema:   &schema.Schema{Type: "string"},
			})
		}
	}
	// Sort the parameters for determinism.
	for _, p := range pathOrder {
		pi := byPath[p]
		sort.SliceStable(pi.Parameters, func(i, j int) bool {
			return pi.Parameters[i].Name < pi.Parameters[j].Name
		})
	}
	sort.Strings(pathOrder)
	out := make([]PathItem, len(pathOrder))
	for i, p := range pathOrder {
		out[i] = *byPath[p]
	}
	return out
}
