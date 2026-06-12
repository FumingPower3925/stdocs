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
//     under the x-stdocs-additionalOperations extension,
//   - host-scoped registrations shadowed in the document (OpenAPI
//     paths cannot express ServeMux host scoping), and
//   - vendor extensions in the output when WithCleanOutput is off.
//
// Lint never affects emission and the findings list may grow in
// future versions; treat it as advice, not a contract. Building the
// document is a side effect, so the same fail-fast panics as
// [Mux.JSON] apply, and like JSON, Lint reads the cached build —
// routes registered after a build need [Mux.Refresh] first.
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
	// The whole walk holds the build lock: building finalizes the
	// routes' operations, and concurrent Lint/JSON/Refresh calls must
	// not observe (or race with) finalize mutating them.
	m.specMu.Lock()
	defer m.specMu.Unlock()
	raw, err := m.cachedJSON()
	if err != nil {
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

	_, shadowed := m.routeVisibility()
	for _, rt := range shadowed {
		where := strings.TrimSpace(rt.parsed.Method + " " + rt.parsed.Path())
		out = append(out, Warning{Where: where,
			Message: "registered on host " + rt.parsed.Host + " but shadowed in the document by another registration of the same method and path (OpenAPI cannot express hosts); the route serves traffic yet is absent from the published contract"})
	}

	report := m.lintComponents()
	names := make([]string, 0, len(report.untyped))
	for name := range report.untyped {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		if report.renamed[name] {
			out = append(out, Warning{Where: "component " + name, Message: "renamed due to a name collision; a SchemaName method gives it a deliberate identifier"})
		}
		for _, field := range report.untyped[name] {
			out = append(out, Warning{Where: "component " + name, Message: "field " + field + " has no schema type; consumers see an unconstrained value (an openapi:\"type=...\" tag pins the wire format)"})
		}
		for _, field := range report.exclusive[name] {
			out = append(out, Warning{Where: "component " + name, Message: "field " + field + " uses an exclusive bound; current Go client generators (ogen, oapi-codegen) reject the numeric 3.1/3.2 form — consumers generating clients should use the 3.0.4 document"})
		}
	}

	if !m.cfg.CleanOutput &&
		(strings.Contains(string(raw), `"x-stdocs-type"`) || strings.Contains(string(raw), `"x-stdocs-warning"`)) {
		out = append(out, Warning{Where: "document", Message: "carries x-stdocs-* annotation extensions; WithCleanOutput(true) strips them from published contracts"})
	}
	return out
}

// lintReport is what lintComponents extracts from a shadow reflection
// of the document's components.
type lintReport struct {
	untyped   map[string][]string // component -> fields with no schema type
	exclusive map[string][]string // component -> fields with exclusive bounds
	renamed   map[string]bool     // components that took a collision suffix
}

// lintComponents rebuilds the same reflection the document build
// performs — route bodies and responses, then webhooks, in the same
// order — so component names, renames, and field findings match the
// emitted document.
func (m *Mux) lintComponents() lintReport {
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
	m.reflectWebhooks(ref)
	report := lintReport{
		untyped:   make(map[string][]string, len(ref.Components())),
		exclusive: make(map[string][]string, len(ref.Components())),
		renamed:   ref.Renamed(),
	}
	exclusiveMatters := m.cfg.Version == OpenAPI31 || m.cfg.Version == OpenAPI32
	for name, comp := range ref.Components() {
		report.untyped[name] = nil
		for _, fieldName := range sortedKeys(comp.Properties) {
			p := comp.Properties[fieldName]
			if p.Type == "" && p.Ref == "" {
				report.untyped[name] = append(report.untyped[name], fieldName)
			}
			if exclusiveMatters && (p.ExclusiveMinimum != "" || p.ExclusiveMaximum != "") {
				report.exclusive[name] = append(report.exclusive[name], fieldName)
			}
		}
	}
	return report
}
