package stdocs

import (
	"sort"
	"strings"
)

// Warning is one advisory finding from [Mux.Lint].
type Warning struct {
	// Code identifies the finding kind stably across releases —
	// allow-lists and CI gates should match on it, never on Message
	// prose, which is free to improve. The codes:
	//
	//	build-failed          the document could not be built
	//	no-error-response     operation declares no 4xx/5xx/default
	//	no-summary            operation has no summary
	//	pattern-approximation the registration cannot be represented
	//	                      exactly (method-less or host-scoped)
	//	shadowed-route        registration absent from the document
	//	name-collision        component renamed with a numeric suffix
	//	untyped-field         schema field with no type
	//	exclusive-bounds      3.1/3.2 exclusive bounds vs generators
	//	nullable-facet-generators
	//	                      nullable + default/uniqueItems/byte vs
	//	                      generators on 3.1/3.2
	//	required-with-default field both required and defaulted
	//	auto-descriptions     "Generated from Go type" text present
	//	dangling-id-suffix    suffixed operationId without a base
	//	vendor-extensions     x-stdocs-* present without CleanOutput
	//
	// Runtime drift findings follow the same discipline in
	// [DriftFinding].Code.
	Code string
	// Where locates the finding: an operation ("GET /tasks"), a
	// component ("component Task"), or "document".
	Where string
	// Message describes the finding and, where obvious, the remedy.
	Message string
}

// String renders the warning as a single log-friendly line.
func (w Warning) String() string { return w.Where + ": " + w.Message + " [" + w.Code + "]" }

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
//   - fields both required and defaulted (a required field can never
//     take its default),
//   - "Generated from Go type" fallback descriptions and stdocs
//     vendor extensions in the output when WithCleanOutput is off,
//     and
//   - operationIds carrying a dangling collision suffix.
//
// Lint never affects emission and the findings list may grow in
// future versions; treat it as advice, not a contract. Building the
// document is a side effect, so the same fail-fast panics as
// [Mux.JSON] apply. Like JSON, Lint rebuilds automatically when
// routes were registered since the last build; [Mux.Refresh] only
// matters for out-of-band changes (e.g. state captured by a
// WithOpenAPI hook).
//
// A CI guard is one assertion:
//
//	accepted := map[string]bool{"no-summary": true} // allow-list by Code
//	for _, w := range mux.Lint() {
//	    if !accepted[w.Code] {
//	        t.Errorf("%s", w)
//	    }
//	}
func (m *Mux) Lint() []Warning {
	// The whole walk holds the build lock: building finalizes the
	// routes' operations, and concurrent Lint/JSON/Refresh calls must
	// not observe (or race with) finalize mutating them.
	m.specMu.Lock()
	defer m.specMu.Unlock()
	raw, err := m.cachedJSON()
	if err != nil {
		return []Warning{{Code: "build-failed", Where: "document", Message: "build failed: " + err.Error()}}
	}
	out := m.lintRoutes()

	report := m.lintComponents()
	names := make([]string, 0, len(report.untyped))
	for name := range report.untyped {
		names = append(names, name)
	}
	sort.Strings(names)
	autoDescribed := false
	for _, name := range names {
		if report.autoDescribed[name] {
			autoDescribed = true
		}
		for _, field := range report.requiredDefaults[name] {
			out = append(out, Warning{Code: "required-with-default", Where: "component " + name,
				Message: "field " + field + " is both required and defaulted — a required field can never take its default; add omitempty or drop the default tag"})
		}
		if report.renamed[name] {
			out = append(out, Warning{Code: "name-collision", Where: "component " + name, Message: "renamed due to a name collision; a SchemaName method gives it a deliberate identifier"})
		}
		for _, field := range report.untyped[name] {
			out = append(out, Warning{Code: "untyped-field", Where: "component " + name, Message: "field " + field + " has no schema type; consumers see an unconstrained value (an openapi:\"type=...\" tag pins the wire format)"})
		}
		for _, field := range report.exclusive[name] {
			out = append(out, Warning{Code: "exclusive-bounds", Where: "component " + name, Message: "field " + field + " uses an exclusive bound; current Go client generators (ogen, oapi-codegen) reject the numeric 3.1/3.2 form — consumers generating clients should use the 3.0.4 document"})
		}
		for _, field := range report.nullableFacets[name] {
			out = append(out, Warning{Code: "nullable-facet-generators", Where: "component " + name, Message: "field " + field + " combines nullability with a default, uniqueItems, or byte format; ogen releases before v1.17.0 reject those combinations in the 3.1/3.2 anyOf form (oapi-codegen consumes 3.0 only) — consumers on older generators should use the 3.0.4 document"})
		}
	}

	if !m.cfg.CleanOutput && autoDescribed {
		out = append(out, Warning{Code: "auto-descriptions", Where: "document", Message: `carries "Generated from Go type ..." fallback descriptions, which leak package layout into published docs; doc: tags or WithCleanOutput(true) replace them`})
	}
	if !m.cfg.CleanOutput &&
		(strings.Contains(string(raw), `"x-stdocs-type"`) || strings.Contains(string(raw), `"x-stdocs-warning"`)) {
		out = append(out, Warning{Code: "vendor-extensions", Where: "document", Message: "carries x-stdocs-* annotation extensions; WithCleanOutput(true) strips them from published contracts"})
	}
	return out
}

// splitIDSuffix reports whether id ends in the _N collision suffix
// and returns the base id.
func splitIDSuffix(id string) (string, bool) {
	i := strings.LastIndexByte(id, '_')
	if i < 0 || i == len(id)-1 {
		return "", false
	}
	for _, r := range id[i+1:] {
		if r < '0' || r > '9' {
			return "", false
		}
	}
	return id[:i], true
}

// lintRoutes produces the per-operation findings — missing error
// responses, missing summaries, pattern approximations, dangling
// operationId suffixes — plus the shadowed-route findings.
func (m *Mux) lintRoutes() []Warning {
	var out []Warning
	visible, shadowed := m.routeVisibility()
	ids := make(map[string]bool, len(visible))
	for _, rt := range visible {
		ids[rt.op.OperationID] = true
	}
	for _, rt := range visible {
		where := strings.TrimSpace(rt.op.Method + " " + rt.parsed.Path())
		hasError := false
		for key := range rt.op.Responses {
			if key == "default" || key[0] == '4' || key[0] == '5' {
				hasError = true
				break
			}
		}
		if !hasError {
			out = append(out, Warning{Code: "no-error-response", Where: where, Message: "documents no error response (no 4xx/5xx and no default entry); consider WithResponse or a mux-level WithDefaultResponse"})
		}
		if rt.op.Summary == "" {
			out = append(out, Warning{Code: "no-summary", Where: where, Message: "has no summary; docs portals and generated clients surface it"})
		}
		if msg, ok := rt.op.Extensions["x-stdocs-warning"].(string); ok {
			out = append(out, Warning{Code: "pattern-approximation", Where: where, Message: msg})
		}
		// A dangling _N id with no base reads as an inexplicable
		// rename to consumers. Auto-derived ids are skipped — a path
		// like /reports/2024 legitimately derives get_reports_2024.
		if rt.op.OperationID == defaultOperationID(rt.parsed) {
			continue
		}
		if base, suffixed := splitIDSuffix(rt.op.OperationID); suffixed && !ids[base] {
			out = append(out, Warning{Code: "dangling-id-suffix", Where: where,
				Message: "operationId " + rt.op.OperationID + " carries a collision suffix but no " + base + " exists in the document; set OperationID explicitly"})
		}
	}
	for _, rt := range shadowed {
		where := strings.TrimSpace(rt.parsed.Method + " " + rt.parsed.Path())
		out = append(out, Warning{Code: "shadowed-route", Where: where,
			Message: "registered on host " + rt.parsed.Host + " but shadowed in the document by another registration of the same method and path (OpenAPI cannot express hosts); the route serves traffic yet is absent from the published contract"})
	}
	return out
}

// lintReport is what lintComponents extracts from a shadow reflection
// of the document's components.
type lintReport struct {
	untyped          map[string][]string // component -> fields with no schema type
	exclusive        map[string][]string // component -> fields with exclusive bounds
	nullableFacets   map[string][]string // component -> nullable fields with generator-hostile facets
	requiredDefaults map[string][]string // component -> fields both required and defaulted
	autoDescribed    map[string]bool     // components carrying the generated fallback description
	renamed          map[string]bool     // components that took a collision suffix
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
		untyped:          make(map[string][]string, len(ref.Components())),
		exclusive:        make(map[string][]string, len(ref.Components())),
		nullableFacets:   make(map[string][]string, len(ref.Components())),
		requiredDefaults: make(map[string][]string, len(ref.Components())),
		autoDescribed:    make(map[string]bool, len(ref.Components())),
		renamed:          ref.Renamed(),
	}
	exclusiveMatters := m.cfg.Version == OpenAPI31 || m.cfg.Version == OpenAPI32
	for name, comp := range ref.Components() {
		report.untyped[name] = nil
		if strings.HasPrefix(comp.Description, "Generated from Go type ") {
			report.autoDescribed[name] = true
		}
		required := make(map[string]bool, len(comp.Required))
		for _, rn := range comp.Required {
			required[rn] = true
		}
		for _, fieldName := range sortedKeys(comp.Properties) {
			p := comp.Properties[fieldName]
			if p.Type == "" && p.Ref == "" {
				report.untyped[name] = append(report.untyped[name], fieldName)
			}
			if exclusiveMatters && (p.ExclusiveMinimum != "" || p.ExclusiveMaximum != "") {
				report.exclusive[name] = append(report.exclusive[name], fieldName)
			}
			if exclusiveMatters && p.Nullable &&
				(p.Default != nil || p.UniqueItems || (p.Type == "string" && p.Format == "byte")) {
				report.nullableFacets[name] = append(report.nullableFacets[name], fieldName)
			}
			if p.Default != nil && required[fieldName] {
				report.requiredDefaults[name] = append(report.requiredDefaults[name], fieldName)
			}
		}
	}
	return report
}
