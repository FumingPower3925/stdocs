package stdocs

import (
	"sort"
	"strings"
)

// Warning is one advisory finding from [Mux.Lint].
type Warning struct {
	// Where locates the finding: an operation ("GET /tasks"), a
	// component ("component Task"), or "document".
	Where string
	// Message describes the finding and, where obvious, the remedy.
	Message string
}

// String renders the warning as a single log-friendly line.
func (w Warning) String() string { return w.Where + ": " + w.Message }

// Lint reports advisory findings about the generated document —
// things that are structurally valid but consume badly downstream
// (client generators, linters, API portals):
//
//   - operations documenting no error response (no 4xx/5xx and no
//     "default" entry),
//   - operations without a summary,
//   - schema fields with no type (interfaces, json.RawMessage,
//     custom marshalers), which consumers see as unconstrained,
//   - component names that needed a collision suffix (User_2) —
//     fixable with a SchemaName method,
//   - custom-method operations, which 3.0/3.1 consumers only see
//     under the x-stdocs-additionalOperations extension, and
//   - vendor extensions in the output when WithCleanOutput is off.
//
// Lint never affects emission and the findings list may grow in
// future versions; treat it as advice, not a contract. Building the
// document is a side effect, so the same fail-fast panics as
// [Mux.JSON] apply.
//
// A CI guard is one assertion:
//
//	if warnings := mux.Lint(); len(warnings) > 0 {
//	    for _, w := range warnings {
//	        t.Log(w)
//	    }
//	    t.Errorf("%d consumability warnings", len(warnings))
//	}
func (m *Mux) Lint() []Warning {
	if _, err := m.JSON(); err != nil {
		return []Warning{{Where: "document", Message: "build failed: " + err.Error()}}
	}
	var out []Warning

	for _, rt := range m.visibleRoutes() {
		where := strings.TrimSpace(rt.op.Method + " " + rt.parsed.Path())
		hasError := false
		for key := range rt.op.Responses {
			if key == "default" || key[0] == '4' || key[0] == '5' {
				hasError = true
				break
			}
		}
		if !hasError {
			out = append(out, Warning{Where: where, Message: "documents no error response (no 4xx/5xx and no default entry); consider WithResponse or a mux-level WithDefaultResponse"})
		}
		if rt.op.Summary == "" {
			out = append(out, Warning{Where: where, Message: "has no summary; docs portals and generated clients surface it"})
		}
		if msg, ok := rt.op.Extensions["x-stdocs-warning"].(string); ok {
			out = append(out, Warning{Where: where, Message: msg})
		}
	}

	comps := m.lintComponents()
	names := make([]string, 0, len(comps))
	for name := range comps {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		if isCollisionSuffixed(name) {
			out = append(out, Warning{Where: "component " + name, Message: "renamed due to a name collision; a SchemaName method gives it a deliberate identifier"})
		}
		for _, field := range comps[name] {
			out = append(out, Warning{Where: "component " + name, Message: "field " + field + " has no schema type; consumers see an unconstrained value (an openapi:\"type=...\" tag pins the wire format)"})
		}
	}

	if !m.cfg.CleanOutput {
		raw, _ := m.JSON()
		if strings.Contains(string(raw), `"x-stdocs-type"`) {
			out = append(out, Warning{Where: "document", Message: "carries x-stdocs-* annotation extensions; WithCleanOutput(true) strips them from published contracts"})
		}
	}
	return out
}

// lintComponents maps each named component to its untyped field
// names, rebuilding the same reflection the document build performs.
func (m *Mux) lintComponents() map[string][]string {
	visible := &registry{routes: m.visibleRoutes()}
	ref := newConfiguredReflector(m.cfg)
	for _, rt := range visible.routes {
		if rb := rt.op.RequestBody; rb != nil && rb.BodyValue != nil {
			ref.Reflect(rb.BodyValue)
		}
		for _, key := range sortedKeys(rt.op.Responses) {
			if resp := rt.op.Responses[key]; resp != nil && resp.BodyValue != nil {
				ref.Reflect(resp.BodyValue)
			}
		}
	}
	out := make(map[string][]string, len(ref.Components()))
	for name, comp := range ref.Components() {
		var untyped []string
		for _, fieldName := range sortedKeys(comp.Properties) {
			p := comp.Properties[fieldName]
			if p.Type == "" && p.Ref == "" {
				untyped = append(untyped, fieldName)
			}
		}
		out[name] = untyped
	}
	return out
}

// isCollisionSuffixed reports whether name ends in the _N collision
// suffix reserveName appends.
func isCollisionSuffixed(name string) bool {
	i := strings.LastIndexByte(name, '_')
	if i < 0 || i == len(name)-1 {
		return false
	}
	for _, r := range name[i+1:] {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
