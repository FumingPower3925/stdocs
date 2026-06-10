package spec

import (
	"encoding/json"
	"sort"
	"strings"

	"github.com/FumingPower3925/stdocs/internal/schema"
	"github.com/FumingPower3925/stdocs/internal/version"
)

// EmitOpenAPI31 produces the OpenAPI 3.1.0 JSON bytes for the given input.
// The output is the 3.0.3 emitter with two adjustments:
//
//   - Nullable is rendered as a type array ("type": ["T", "null"]) instead
//     of a "nullable": true sibling.
//   - The top-level "openapi" field is "3.1.0".
//
// All other structure is identical to 3.0.3.
func EmitOpenAPI31(in SpecInput) ([]byte, error) {
	doc := BuildRoot31(in)
	b, err := json.Marshal(doc)
	if err != nil {
		return nil, err
	}
	return b, nil
}

func BuildRoot31(in SpecInput) map[string]any {
	doc := map[string]any{
		"openapi": string(version.OpenAPI31),
		"info":    buildInfo30(in.Info), // identical structure
	}
	if servers := buildServers30(in.Servers); servers != nil {
		doc["servers"] = servers
	}
	if tags := buildTags30(in.Tags); tags != nil {
		doc["tags"] = tags
	}
	doc["paths"] = buildPaths31(in.Paths)
	doc["components"] = buildComponents31WithSecurity(in.Components, in.SecuritySchemes)
	if len(in.GlobalSecurity) > 0 {
		doc["security"] = buildSecurity(in.GlobalSecurity)
	}
	if len(in.Webhooks) > 0 {
		doc["webhooks"] = buildWebhooks31(in.Webhooks)
	}
	return doc
}

// buildWebhooks31 turns the Webhooks map into the OpenAPI 3.1 "webhooks"
// object: a map keyed by webhook name, where each value is a map of
// HTTP method to Operation.
func buildWebhooks31(hooks map[string]Webhook) map[string]any {
	out := make(map[string]any, len(hooks))
	// Sort names for determinism.
	names := make([]string, 0, len(hooks))
	for n := range hooks {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, name := range names {
		hook := hooks[name]
		method := strings.ToLower(hook.Method)
		op := buildOperationFromHook31(hook)
		out[name] = map[string]any{method: op}
	}
	return out
}

// buildOperationFromHook31 builds a single operation map for a webhook.
func buildOperationFromHook31(hook Webhook) map[string]any {
	m := map[string]any{}
	if hook.Summary != "" {
		m["summary"] = hook.Summary
	}
	if hook.Description != "" {
		m["description"] = hook.Description
	}
	if hook.OperationID != "" {
		m["operationId"] = hook.OperationID
	}
	if hook.RequestBody != nil {
		m["requestBody"] = buildRequestBody31(hook.RequestBody)
	}
	if len(hook.Responses) > 0 {
		m["responses"] = buildResponses31(hook.Responses)
	} else {
		m["responses"] = map[string]any{
			"200": map[string]any{"description": "OK"},
		}
	}
	return m
}

func buildPaths31(paths []PathItem) map[string]any {
	out := make(map[string]any, len(paths))
	sorted := make([]PathItem, len(paths))
	copy(sorted, paths)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Path < sorted[j].Path })
	for _, p := range sorted {
		out[p.Path] = buildPathItem31(p)
	}
	return out
}

func buildPathItem31(p PathItem) map[string]any {
	m := make(map[string]any)
	if len(p.Parameters) > 0 {
		params := make([]any, 0, len(p.Parameters))
		for _, pa := range p.Parameters {
			params = append(params, buildParam31(pa))
		}
		m["parameters"] = params
	}
	methods := make([]string, 0, len(p.Operations))
	for k := range p.Operations {
		methods = append(methods, k)
	}
	sort.Strings(methods)
	for _, method := range methods {
		m[method] = buildOperation31(p.Operations[method])
	}
	return m
}

func buildParam31(p Param) map[string]any {
	m := map[string]any{
		"name": p.Name,
		"in":   p.In,
	}
	if p.Schema != nil {
		m["schema"] = buildSchema31(p.Schema)
	}
	if p.Description != "" {
		m["description"] = p.Description
	}
	if p.Required || p.In == "path" {
		m["required"] = true
	}
	return m
}

func buildOperation31(op *Operation) map[string]any {
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
			params = append(params, buildParam31(pa))
		}
		m["parameters"] = params
	}
	if op.RequestBody != nil {
		m["requestBody"] = buildRequestBody31(op.RequestBody)
	}
	if len(op.Responses) > 0 {
		m["responses"] = buildResponses31(op.Responses)
	} else {
		m["responses"] = map[string]any{
			"200": map[string]any{"description": "OK"},
		}
	}
	if len(op.Security) > 0 {
		m["security"] = buildSecurity(op.Security)
	}
	return m
}

func buildRequestBody31(rb *RequestBody) map[string]any {
	ct := rb.ContentType
	if ct == "" {
		ct = "application/json"
	}
	contentEntry := map[string]any{
		"schema": buildSchema31(rb.Schema),
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

func buildResponses31(responses map[string]*Response) map[string]any {
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
				"schema": buildSchema31(r.Schema),
			}
			if r.Example != nil {
				contentEntry["example"] = r.Example
			}
			m["content"] = map[string]any{
				"application/json": contentEntry,
			}
		} else if r.Example != nil {
			m["content"] = map[string]any{
				"application/json": map[string]any{
					"example": r.Example,
				},
			}
		}
		if len(r.Headers) > 0 {
			hdrs := make(map[string]any, len(r.Headers))
			for name, sch := range r.Headers {
				hdrs[name] = map[string]any{"schema": buildSchema31(sch)}
			}
			m["headers"] = hdrs
		}
		out[status] = m
	}
	return out
}

func buildComponents31(components map[string]*schema.Schema) map[string]any {
	// Always emit components so user hooks (WithOpenAPI) can rely on
	// the field being present. An empty "schemas" is valid per spec.
	names := make([]string, 0, len(components))
	for n := range components {
		names = append(names, n)
	}
	sort.Strings(names)
	schemas := make(map[string]any, len(components))
	for _, n := range names {
		schemas[n] = buildSchema31(components[n])
	}
	return map[string]any{"schemas": schemas}
}

// buildComponents31WithSecurity builds the components block including
// both schemas and security schemes.
func buildComponents31WithSecurity(components map[string]*schema.Schema, schemes []NamedSecurityScheme) map[string]any {
	out := buildComponents31(components)
	if sec := buildSecuritySchemes(schemes); sec != nil {
		out["securitySchemes"] = sec
	}
	return out
}

// buildSchema31 converts a *schema.Schema into the map[string]any form for 3.1.0.
// The Nullable field is NOT emitted; the reflector's applyVersion converts
// it into a TypeArray ("type": ["T", "null"]) in 3.1 mode.
func buildSchema31(s *schema.Schema) map[string]any {
	if s == nil {
		return nil
	}
	if s.Ref != "" {
		return map[string]any{"$ref": s.Ref}
	}
	m := make(map[string]any)
	// Prefer TypeArray if set (3.1 nullable form).
	if len(s.TypeArray) > 0 {
		m["type"] = s.TypeArray
	} else if s.Type != "" {
		m["type"] = s.Type
	}
	if s.Format != "" {
		m["format"] = s.Format
	}
	if s.Description != "" {
		m["description"] = s.Description
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
				props[k] = buildSchema31(s.Properties[k])
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
			m["additionalProperties"] = buildSchema31(s.AdditionalProperties)
		}
	}
	if s.Type == "array" && s.Items != nil {
		m["items"] = buildSchema31(s.Items)
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
	for k, v := range s.Extensions {
		m[k] = v
	}
	if len(m) == 0 {
		return map[string]any{}
	}
	return m
}
