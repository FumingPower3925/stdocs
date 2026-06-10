package spec

import (
	"encoding/json"
	"sort"

	"github.com/FumingPower3925/stdocs/internal/schema"
	"github.com/FumingPower3925/stdocs/internal/version"
)

// EmitOpenAPI30 produces the OpenAPI 3.0.3 JSON bytes for the given input.
// The result is sorted by key in every nested object so the output is
// deterministic across runs (useful for tests and golden files).
func EmitOpenAPI30(in SpecInput) ([]byte, error) {
	root := BuildRoot30(in)
	// Marshal with stable key order via a custom encoder.
	b, err := json.Marshal(root)
	if err != nil {
		return nil, err
	}
	return b, nil
}

func BuildRoot30(in SpecInput) map[string]any {
	doc := map[string]any{
		"openapi": string(version.OpenAPI30),
		"info":    buildInfo30(in.Info),
	}
	if servers := buildServers30(in.Servers); servers != nil {
		doc["servers"] = servers
	}
	if tags := buildTags30(in.Tags); tags != nil {
		doc["tags"] = tags
	}
	doc["paths"] = buildPaths30(in.Paths)
	doc["components"] = buildComponents30WithSecurity(in.Components, in.SecuritySchemes)
	if len(in.GlobalSecurity) > 0 {
		doc["security"] = buildSecurity(in.GlobalSecurity)
	}
	return doc
}

func buildInfo30(i Info) map[string]any {
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
		m["contact"] = map[string]any{
			"name":  i.Contact.Name,
			"url":   i.Contact.URL,
			"email": i.Contact.Email,
		}
	}
	if i.License != nil && i.License.Name != "" {
		m["license"] = map[string]any{
			"name": i.License.Name,
			"url":  i.License.URL,
		}
	}
	return m
}

func buildServers30(servers []Server) []any {
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

func buildTags30(tags []TagDecl) []any {
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

func buildPaths30(paths []PathItem) map[string]any {
	out := make(map[string]any, len(paths))
	// Sort paths for determinism.
	sorted := make([]PathItem, len(paths))
	copy(sorted, paths)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Path < sorted[j].Path })
	for _, p := range sorted {
		out[p.Path] = buildPathItem30(p)
	}
	return out
}

func buildPathItem30(p PathItem) map[string]any {
	m := make(map[string]any)
	if len(p.Parameters) > 0 {
		params := make([]any, 0, len(p.Parameters))
		for _, pa := range p.Parameters {
			params = append(params, buildParam30(pa))
		}
		m["parameters"] = params
	}
	// Sort methods for determinism.
	methods := make([]string, 0, len(p.Operations))
	for k := range p.Operations {
		methods = append(methods, k)
	}
	sort.Strings(methods)
	for _, method := range methods {
		m[method] = buildOperation30(p.Operations[method])
	}
	return m
}

func buildParam30(p Param) map[string]any {
	m := map[string]any{
		"name": p.Name,
		"in":   p.In,
	}
	if p.Schema != nil {
		m["schema"] = buildSchema30(p.Schema)
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

func buildOperation30(op *Operation) map[string]any {
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
			params = append(params, buildParam30(pa))
		}
		m["parameters"] = params
	}
	if op.RequestBody != nil {
		m["requestBody"] = buildRequestBody30(op.RequestBody)
	}
	if len(op.Responses) > 0 {
		m["responses"] = buildResponses30(op.Responses)
	} else {
		// OpenAPI requires a responses object. If the user gave us nothing,
		// emit the auto-200.
		m["responses"] = map[string]any{
			"200": map[string]any{"description": "OK"},
		}
	}
	if len(op.Security) > 0 {
		m["security"] = buildSecurity(op.Security)
	}
	return m
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

func buildRequestBody30(rb *RequestBody) map[string]any {
	ct := rb.ContentType
	if ct == "" {
		ct = "application/json"
	}
	contentEntry := map[string]any{
		"schema": buildSchema30(rb.Schema),
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

func buildResponses30(responses map[string]*Response) map[string]any {
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
				"schema": buildSchema30(r.Schema),
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
				hdrs[name] = map[string]any{"schema": buildSchema30(sch)}
			}
			m["headers"] = hdrs
		}
		out[status] = m
	}
	return out
}

func buildComponents30(components map[string]*schema.Schema) map[string]any {
	// Always emit components so user hooks (WithOpenAPI) can rely on
	// the field being present. An empty "schemas" is valid per spec.
	names := make([]string, 0, len(components))
	for n := range components {
		names = append(names, n)
	}
	sort.Strings(names)
	schemas := make(map[string]any, len(components))
	for _, n := range names {
		schemas[n] = buildSchema30(components[n])
	}
	return map[string]any{"schemas": schemas}
}

// buildComponents30WithSecurity builds the components block including
// both schemas and security schemes.
func buildComponents30WithSecurity(components map[string]*schema.Schema, schemes []NamedSecurityScheme) map[string]any {
	out := buildComponents30(components)
	if sec := buildSecuritySchemes(schemes); sec != nil {
		out["securitySchemes"] = sec
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
		if s.Scheme != "" {
			m["scheme"] = s.Scheme
		}
		if s.BearerFormat != "" {
			m["bearerFormat"] = s.BearerFormat
		}
	case SecurityAPIKey:
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
	if f.AuthorizationURL != "" {
		m["authorizationUrl"] = f.AuthorizationURL
	}
	if f.TokenURL != "" {
		m["tokenUrl"] = f.TokenURL
	}
	if f.RefreshURL != "" {
		m["refreshUrl"] = f.RefreshURL
	}
	if len(f.Scopes) > 0 {
		// Sort scopes for determinism.
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
	}
	return m
}

// buildSchema30 converts a *schema.Schema into the map[string]any form for 3.0.3.
// Nullable is rendered as "nullable": true. TypeArray is ignored (3.0.3
// has no concept of type arrays).
func buildSchema30(s *schema.Schema) map[string]any {
	if s == nil {
		return nil
	}
	// $ref is the simplest case: emit just the ref.
	if s.Ref != "" {
		return map[string]any{"$ref": s.Ref}
	}
	m := make(map[string]any)
	if s.Type != "" {
		m["type"] = s.Type
	}
	if s.Format != "" {
		m["format"] = s.Format
	}
	if s.Description != "" {
		m["description"] = s.Description
	}
	if s.Nullable {
		m["nullable"] = true
	}
	if s.Type == "object" {
		if len(s.Properties) > 0 {
			props := make(map[string]any, len(s.Properties))
			keys := make([]string, 0, len(s.Properties))
			for k := range s.Properties {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				props[k] = buildSchema30(s.Properties[k])
			}
			m["properties"] = props
		}
		if len(s.Required) > 0 {
			req := make([]string, len(s.Required))
			copy(req, s.Required)
			sort.Strings(req)
			m["required"] = req
		}
		if s.AdditionalProperties != nil {
			m["additionalProperties"] = buildSchema30(s.AdditionalProperties)
		}
	}
	if s.Type == "array" && s.Items != nil {
		m["items"] = buildSchema30(s.Items)
	}
	if len(s.Enum) > 0 {
		m["enum"] = s.Enum
	}
	if s.Default != nil {
		m["default"] = s.Default
	}
	if s.Example != nil {
		m["example"] = s.Example
	}
	// Extensions (x-*).
	for k, v := range s.Extensions {
		m[k] = v
	}
	if len(m) == 0 {
		// A truly empty schema — emit {} to make JSON happy.
		return map[string]any{}
	}
	return m
}
