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
}

// defaultOperationID builds an operationId like "get_users_id" from a
// parsed pattern. The method lower-cased, then the path segments joined
// by underscores, with wildcards keeping their names.
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
		case pattern.KindWildcard, pattern.KindMulti:
			v = s.Value
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
