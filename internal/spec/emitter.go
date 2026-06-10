// Package spec builds OpenAPI 3.0.3 and 3.1.0 documents from
// internal *spec.SpecInput values. The two emitters (BuildRoot30
// and BuildRoot31) share a single emitter struct that handles
// info, servers, tags, paths, components, security, and
// webhooks; only the schema builder and version string differ
// between versions.
//
// The input types (Operation, PathItem, Param, RequestBody,
// Response, Info, etc.) are version-agnostic. The package also
// re-exports the security-related types so callers can build a
// complete spec from a single import.
package spec

import (
	"sort"
	"strings"

	"github.com/FumingPower3925/stdocs/internal/schema"
)

// emitter groups the version-specific bits of OpenAPI emission. The
// shape of an OpenAPI 3.0.3 spec and a 3.1.0 spec is the same for
// everything except schema and webhooks; this struct lets us share
// the bulk of the build code between versions without resorting to
// copy-paste.
type emitter struct {
	openapi     string
	buildSchema func(*schema.Schema) map[string]any
}

// buildRoot assembles the top-level OpenAPI object. Webhooks are
// only emitted for 3.1; on 3.0.3 they are silently dropped because
// the field is not part of the 3.0.3 spec. The caller does not need
// to filter in.Webhooks.
func (e *emitter) buildRoot(in SpecInput) map[string]any {
	doc := map[string]any{
		"openapi": e.openapi,
		"info":    e.buildInfo(in.Info),
	}
	if servers := e.buildServers(in.Servers); servers != nil {
		doc["servers"] = servers
	}
	if tags := e.buildTags(in.Tags); tags != nil {
		doc["tags"] = tags
	}
	doc["paths"] = e.buildPaths(in.Paths)
	doc["components"] = e.buildComponentsWithSecurity(in.Components, in.SecuritySchemes)
	if len(in.GlobalSecurity) > 0 {
		doc["security"] = buildSecurity(in.GlobalSecurity)
	}
	if len(in.Webhooks) > 0 && strings.HasPrefix(e.openapi, "3.1") {
		doc["webhooks"] = e.buildWebhooks(in.Webhooks)
	}
	return doc
}

func (e *emitter) buildInfo(i Info) map[string]any {
	m := map[string]any{
		"title":   i.Title,
		"version": i.Version,
	}
	if i.Description != "" {
		m["description"] = i.Description
	}
	if i.TermsOfService != "" {
		m["termsOfService"] = i.TermsOfService
	}
	if i.Contact != nil && (i.Contact.Name != "" || i.Contact.URL != "" || i.Contact.Email != "") {
		c := map[string]any{}
		if i.Contact.Name != "" {
			c["name"] = i.Contact.Name
		}
		if i.Contact.URL != "" {
			c["url"] = i.Contact.URL
		}
		if i.Contact.Email != "" {
			c["email"] = i.Contact.Email
		}
		m["contact"] = c
	}
	if i.License != nil && (i.License.Name != "" || i.License.URL != "") {
		l := map[string]any{}
		if i.License.Name != "" {
			l["name"] = i.License.Name
		}
		if i.License.URL != "" {
			l["url"] = i.License.URL
		}
		m["license"] = l
	}
	return m
}

func (e *emitter) buildServers(servers []Server) []any {
	if len(servers) == 0 {
		return nil
	}
	out := make([]any, 0, len(servers))
	for _, s := range servers {
		m := map[string]any{"url": s.URL}
		if s.Description != "" {
			m["description"] = s.Description
		}
		out = append(out, m)
	}
	return out
}

func (e *emitter) buildTags(tags []TagDecl) []any {
	if len(tags) == 0 {
		return nil
	}
	out := make([]any, 0, len(tags))
	for _, t := range tags {
		m := map[string]any{"name": t.Name}
		if t.Description != "" {
			m["description"] = t.Description
		}
		out = append(out, m)
	}
	return out
}

func (e *emitter) buildPaths(paths []PathItem) map[string]any {
	out := make(map[string]any, len(paths))
	sorted := make([]PathItem, len(paths))
	copy(sorted, paths)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Path < sorted[j].Path })
	for _, p := range sorted {
		out[p.Path] = e.buildPathItem(p)
	}
	return out
}

func (e *emitter) buildPathItem(p PathItem) map[string]any {
	m := make(map[string]any)
	if len(p.Parameters) > 0 {
		params := make([]any, 0, len(p.Parameters))
		for _, pa := range p.Parameters {
			params = append(params, e.buildParam(pa))
		}
		m["parameters"] = params
	}
	methods := make([]string, 0, len(p.Operations))
	for k := range p.Operations {
		methods = append(methods, k)
	}
	sort.Strings(methods)
	for _, method := range methods {
		m[method] = e.buildOperation(p.Operations[method])
	}
	return m
}

func (e *emitter) buildParam(p Param) map[string]any {
	m := map[string]any{
		"name": p.Name,
		"in":   p.In,
	}
	if p.Schema != nil {
		m["schema"] = e.buildSchema(p.Schema)
	}
	if p.Description != "" {
		m["description"] = p.Description
	}
	if p.Required || p.In == "path" {
		// Path params are always required per spec; we always set this.
		m["required"] = true
	}
	return m
}

func (e *emitter) buildOperation(op *Operation) map[string]any {
	m := map[string]any{}
	if op.Summary != "" {
		m["summary"] = op.Summary
	}
	if op.Description != "" {
		m["description"] = op.Description
	}
	if op.OperationID != "" {
		m["operationId"] = op.OperationID
	}
	if len(op.Tags) > 0 {
		// Deduplicate and sort for determinism.
		seen := make(map[string]bool)
		tags := make([]string, 0, len(op.Tags))
		for _, t := range op.Tags {
			if !seen[t] {
				seen[t] = true
				tags = append(tags, t)
			}
		}
		sort.Strings(tags)
		m["tags"] = tags
	}
	if op.Deprecated {
		m["deprecated"] = true
	}
	if len(op.Parameters) > 0 {
		params := make([]any, 0, len(op.Parameters))
		for _, pa := range op.Parameters {
			params = append(params, e.buildParam(pa))
		}
		m["parameters"] = params
	}
	if op.RequestBody != nil {
		m["requestBody"] = e.buildRequestBody(op.RequestBody)
	}
	if len(op.Responses) > 0 {
		m["responses"] = e.buildResponses(op.Responses)
	} else {
		// OpenAPI requires a responses object. If the user gave us nothing,
		// emit the auto-200.
		m["responses"] = map[string]any{
			"200": map[string]any{"description": "OK"},
		}
	}
	switch {
	case op.NoSecurity:
		// Explicit opt-out. An empty array overrides the global
		// security requirement at this operation.
		m["security"] = []any{}
	case len(op.Security) > 0:
		m["security"] = buildSecurity(op.Security)
	}
	for k, v := range op.Extensions {
		m[k] = v
	}
	return m
}

func (e *emitter) buildRequestBody(rb *RequestBody) map[string]any {
	ct := rb.ContentType
	if ct == "" {
		ct = "application/json"
	}
	contentEntry := map[string]any{
		"schema": e.buildSchema(rb.Schema),
	}
	if rb.Example != nil {
		contentEntry["example"] = rb.Example
	}
	m := map[string]any{
		"content": map[string]any{
			ct: contentEntry,
		},
	}
	if rb.Description != "" {
		m["description"] = rb.Description
	}
	if rb.Required {
		m["required"] = true
	}
	return m
}

func (e *emitter) buildResponses(responses map[string]*Response) map[string]any {
	out := make(map[string]any, len(responses))
	statuses := make([]string, 0, len(responses))
	for k := range responses {
		statuses = append(statuses, k)
	}
	sort.Strings(statuses)
	for _, status := range statuses {
		r := responses[status]
		m := map[string]any{
			"description": r.Description,
		}
		if r.Description == "" {
			m["description"] = "Response"
		}
		if r.Schema != nil {
			contentEntry := map[string]any{
				"schema": e.buildSchema(r.Schema),
			}
			if r.Example != nil {
				contentEntry["example"] = r.Example
			}
			m["content"] = map[string]any{
				"application/json": contentEntry,
			}
		} else if r.Example != nil {
			// Example with no schema: emit a synthetic content entry
			// so the example isn't lost.
			m["content"] = map[string]any{
				"application/json": map[string]any{
					"example": r.Example,
				},
			}
		}
		if len(r.Headers) > 0 {
			hdrs := make(map[string]any, len(r.Headers))
			for name, sch := range r.Headers {
				hdrs[name] = map[string]any{"schema": e.buildSchema(sch)}
			}
			m["headers"] = hdrs
		}
		out[status] = m
	}
	return out
}

func (e *emitter) buildComponents(components map[string]*schema.Schema) map[string]any {
	// Always emit components so user hooks (WithOpenAPI) can rely on
	// the field being present. An empty "schemas" is valid per spec.
	names := make([]string, 0, len(components))
	for n := range components {
		names = append(names, n)
	}
	sort.Strings(names)
	schemas := make(map[string]any, len(components))
	for _, n := range names {
		schemas[n] = e.buildSchema(components[n])
	}
	return map[string]any{"schemas": schemas}
}

// buildComponentsWithSecurity builds the components block including
// both schemas and security schemes.
func (e *emitter) buildComponentsWithSecurity(components map[string]*schema.Schema, schemes []NamedSecurityScheme) map[string]any {
	out := e.buildComponents(components)
	if sec := buildSecuritySchemes(schemes); sec != nil {
		out["securitySchemes"] = sec
	}
	return out
}

// buildWebhooks turns the Webhooks map into the OpenAPI 3.1 "webhooks"
// object. (3.0.3 does not have webhooks; the caller decides whether
// to invoke this method by checking in.Webhooks.)
func (e *emitter) buildWebhooks(hooks map[string]Webhook) map[string]any {
	out := make(map[string]any, len(hooks))
	names := make([]string, 0, len(hooks))
	for n := range hooks {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, name := range names {
		hook := hooks[name]
		method := strings.ToLower(hook.Method)
		op := map[string]any{}
		if hook.Summary != "" {
			op["summary"] = hook.Summary
		}
		if hook.Description != "" {
			op["description"] = hook.Description
		}
		if hook.OperationID != "" {
			op["operationId"] = hook.OperationID
		}
		if hook.RequestBody != nil {
			op["requestBody"] = e.buildRequestBody(hook.RequestBody)
		}
		if len(hook.Responses) > 0 {
			op["responses"] = e.buildResponses(hook.Responses)
		} else {
			op["responses"] = map[string]any{
				"200": map[string]any{"description": "OK"},
			}
		}
		out[name] = map[string]any{method: op}
	}
	return out
}

// buildSecurity turns a slice of SecurityRequirement into the OpenAPI
// "security" array form.
func buildSecurity(reqs []SecurityRequirement) []any {
	out := make([]any, 0, len(reqs))
	for _, r := range reqs {
		entry := make(map[string]any, len(r))
		for name, scopes := range r {
			if len(scopes) == 0 {
				entry[name] = []string{}
			} else {
				entry[name] = scopes
			}
		}
		out = append(out, entry)
	}
	return out
}

// buildSecuritySchemes turns a slice of named security schemes into
// the OpenAPI "components.securitySchemes" map.
func buildSecuritySchemes(schemes []NamedSecurityScheme) map[string]any {
	if len(schemes) == 0 {
		return nil
	}
	out := make(map[string]any, len(schemes))
	for _, s := range schemes {
		out[s.Name] = buildSecurityScheme(s.Scheme)
	}
	return out
}

// buildSecurityScheme converts a SecurityScheme into its map form.
func buildSecurityScheme(s SecurityScheme) map[string]any {
	m := map[string]any{
		"type": string(s.Type),
	}
	if s.Description != "" {
		m["description"] = s.Description
	}
	switch s.Type {
	case SecurityHTTP:
		// `scheme` is required for "http" type. The stddocs API does
		// not allow constructing an "http" scheme without setting
		// it (e.g. WithBearerAuth sets "bearer", WithBasicAuth sets
		// "basic"), so emitting unconditionally is correct.
		if s.Scheme != "" {
			m["scheme"] = s.Scheme
		}
		if s.BearerFormat != "" {
			m["bearerFormat"] = s.BearerFormat
		}
	case SecurityAPIKey:
		// `in` and `name` are both required for "apiKey". Same
		// reasoning: the stddocs API requires them at construction.
		if s.In != "" {
			m["in"] = s.In
		}
		if s.Name != "" {
			m["name"] = s.Name
		}
	case SecurityOAuth2:
		if s.Flows != nil {
			m["flows"] = buildOAuthFlows(*s.Flows)
		}
	case SecurityOpenIDConnect:
		if s.OpenIDConnectURL != "" {
			m["openIdConnectUrl"] = s.OpenIDConnectURL
		}
	}
	return m
}

// buildOAuthFlows turns OAuthFlows into its map form.
func buildOAuthFlows(f OAuthFlows) map[string]any {
	out := map[string]any{}
	if f.Implicit != nil {
		out["implicit"] = buildOAuthFlow(*f.Implicit)
	}
	if f.Password != nil {
		out["password"] = buildOAuthFlow(*f.Password)
	}
	if f.ClientCredentials != nil {
		out["clientCredentials"] = buildOAuthFlow(*f.ClientCredentials)
	}
	if f.AuthorizationCode != nil {
		out["authorizationCode"] = buildOAuthFlow(*f.AuthorizationCode)
	}
	return out
}

func buildOAuthFlow(f OAuthFlow) map[string]any {
	m := map[string]any{}
	// Per the OpenAPI 3.0.3 spec, `scopes` is a required field
	// on every flow object (it may be empty but must be present).
	// The `authorizationUrl` and `tokenUrl` fields are required on
	// specific flows, but only when the flow type requires them;
	// we still emit them only when set to avoid producing
	// misconfigured specs for flows that don't need them.
	keys := make([]string, 0, len(f.Scopes))
	for k := range f.Scopes {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	scopes := make(map[string]string, len(f.Scopes))
	for _, k := range keys {
		scopes[k] = f.Scopes[k]
	}
	m["scopes"] = scopes
	if f.AuthorizationURL != "" {
		m["authorizationUrl"] = f.AuthorizationURL
	}
	if f.TokenURL != "" {
		m["tokenUrl"] = f.TokenURL
	}
	if f.RefreshURL != "" {
		m["refreshUrl"] = f.RefreshURL
	}
	return m
}
