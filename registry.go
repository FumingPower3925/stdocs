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
// config (auto-200, default tag, default summary, operationId) and
// then makes operation ids document-wide unique. Path parameters
// derived from pattern wildcards are emitted once, at the path-item
// level (see toPathItems); operation-level Parameters hold only what
// the user added via WithParam — an operation-level "path" param with
// the same name as a wildcard deliberately overrides the inherited
// one, per the OpenAPI parameter-override rules.
func (r *registry) finalize(cfg *Config) {
	for _, rt := range r.routes {
		applyRouteDefaults(rt, cfg)
	}
	r.disambiguateOperationIDs()
}

// applyRouteDefaults fills the operation fields the user left unset.
func applyRouteDefaults(rt *route, cfg *Config) {
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
	applyMethodWarnings(rt)

	// Default summary: from function name, or from DefaultSummary
	// template, but only if Summary was not provided. The template's
	// {resource} placeholder is replaced with the first path segment,
	// e.g. "List {resource}" for GET /users becomes "List users".
	if rt.op.Summary == "" {
		if s := summaryFromFuncName(rt.funcName); s != "" {
			rt.op.Summary = s
		} else if cfg.DefaultSummary != "" {
			rt.op.Summary = strings.ReplaceAll(cfg.DefaultSummary, "{resource}", firstSegment(rt.pattern))
		}
	}

	// Default tag: from first path segment, but only if no tags
	// were provided. When a tag declared via WithTag matches
	// case-insensitively, adopt the declared casing so the
	// operation groups under the described tag in UIs (inferred
	// "Health" must not split from a declared "health").
	if len(rt.op.Tags) == 0 {
		if tag := tagFromPath(rt.pattern); tag != "" {
			for _, decl := range cfg.Tags {
				if strings.EqualFold(decl.Name, tag) {
					tag = decl.Name
					break
				}
			}
			rt.op.Tags = []string{tag}
		}
	}

	// Default operationId from method+path.
	if rt.op.OperationID == "" {
		rt.op.OperationID = defaultOperationID(rt.parsed)
	}
}

// applyMethodWarnings records x-stdocs-warning extensions for methods
// the OpenAPI version cannot represent as first-class Path Item keys.
//
// Custom HTTP methods (PURGE, etc.) are valid in stdlib ServeMux but
// are not legal as keys of a 3.0/3.1 Path Item Object. The emitter
// places them under the x-stdocs-additionalOperations extension for
// 3.0/3.1 (always legal) and under the standard additionalOperations
// field for 3.2; QUERY is a first-class key in 3.2, so neither gets a
// warning there. Method-less patterns match every HTTP method at
// runtime but are documented as GET only — that choice is surfaced
// too.
func applyMethodWarnings(rt *route) {
	warn := func(msg string) {
		if rt.op.Extensions == nil {
			rt.op.Extensions = map[string]any{}
		}
		rt.op.Extensions["x-stdocs-warning"] = msg
	}
	switch {
	case rt.parsed.Method == "":
		warn("pattern has no method and matches every HTTP method at runtime; it is documented as GET only")
	case !operationKeyIsStandard(rt.parsed.Method, rt.version) && rt.version != OpenAPI32:
		warn("method " + rt.parsed.Method + " is not a legal OpenAPI " + string(rt.version) +
			" Path Item key; the operation is emitted under the x-stdocs-additionalOperations extension")
	}
}

// disambiguateOperationIDs makes operation ids document-wide unique
// with numeric suffixes: the first route with a given id keeps it,
// later ones become "id_2", "id_3"... A generated candidate is never
// an id that any route carries (taken) or that this pass has already
// handed out (used), so an explicit OperationID("x_2") can never end
// up duplicated. Because renames only happen on actual collisions,
// re-running on an already-unique set (e.g. after Refresh) is a
// no-op, keeping ids stable across rebuilds.
func (r *registry) disambiguateOperationIDs() {
	taken := make(map[string]bool, len(r.routes))
	for _, rt := range r.routes {
		taken[rt.op.OperationID] = true
	}
	used := make(map[string]bool, len(r.routes))
	for _, rt := range r.routes {
		id := rt.op.OperationID
		if !used[id] {
			used[id] = true
			continue
		}
		for i := 2; ; i++ {
			cand := id + "_" + itoa(i)
			if !taken[cand] && !used[cand] {
				rt.op.OperationID = cand
				used[cand] = true
				break
			}
		}
	}
}

// operationKeyIsStandard reports whether the (upper-case) method maps
// to a fixed operation key of the OpenAPI Path Item Object for the
// given version: the classic eight for 3.0/3.1, plus QUERY for 3.2.
func operationKeyIsStandard(method string, v SpecVersion) bool {
	if pattern.IsOpenAPIMethod(method) {
		return true
	}
	return v == OpenAPI32 && method == "QUERY"
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
	return strings.Join(parts, "_")
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
