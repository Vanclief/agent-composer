package mcp

import (
	"encoding/json"
	"strings"

	mcpproto "github.com/mark3labs/mcp-go/mcp"
	"github.com/vanclief/ez"
)

func extractToolSchema(tool mcpproto.Tool) (map[string]any, error) {
	const op = "mcp.extractToolSchema"

	// Prefer the SDK-emitted effective schema (includes RawInputSchema when present).
	type toolEnvelope struct {
		InputSchema any `json:"inputSchema"`
	}

	toolJSON, err := json.Marshal(tool)
	if err != nil {
		return nil, ez.New(op, ez.EINTERNAL, "failed to marshal MCP tool to JSON", err)
	}

	var envelope toolEnvelope
	err = json.Unmarshal(toolJSON, &envelope)
	if err != nil {
		return nil, ez.New(op, ez.EINTERNAL, "failed to unmarshal MCP tool JSON", err)
	}

	if envelope.InputSchema != nil {
		envelopeSchema, err := toMap(envelope.InputSchema)
		if err == nil && len(envelopeSchema) > 0 {
			return envelopeSchema, nil
		}
		if err != nil {
			return nil, ez.New(op, ez.EINVALID, "invalid envelope inputSchema: not a JSON object", err)
		}
	}

	// Fallback: try the struct field as is (“it’s already JSON” path).
	rawSchema, err := toMap(tool.InputSchema)
	if err == nil && len(rawSchema) > 0 {
		return rawSchema, nil
	}
	if err != nil {
		return nil, ez.New(op, ez.EINVALID, "invalid tool inputSchema: not a JSON object", err)
	}

	return nil, ez.New(op, ez.ENOTFOUND, "tool inputSchema is missing", nil)
}

func toMap(value any) (map[string]any, error) {
	const op = "mcp.toMap"

	if value == nil {
		return nil, ez.New(op, ez.EINVALID, "nil value", nil)
	}

	existingMap, isMapStringAny := value.(map[string]any)
	if isMapStringAny {
		if len(existingMap) == 0 {
			return nil, ez.New(op, ez.EINVALID, "value is an empty JSON object", nil)
		}
		return existingMap, nil
	}

	rawMessage, isRaw := value.(json.RawMessage)
	if isRaw {
		var schema map[string]any
		err := json.Unmarshal(rawMessage, &schema)
		if err != nil {
			return nil, ez.New(op, ez.EINVALID, "failed to decode json.RawMessage into object", err)
		}
		if len(schema) == 0 {
			return nil, ez.New(op, ez.EINVALID, "value is an empty JSON object", nil)
		}
		return schema, nil
	}

	// Generic: marshal then unmarshal into a map
	jsonBytes, err := json.Marshal(value)
	if err != nil {
		return nil, ez.New(op, ez.EINTERNAL, "failed to marshal value to JSON", err)
	}

	var schema map[string]any
	err = json.Unmarshal(jsonBytes, &schema)
	if err != nil {
		return nil, ez.New(op, ez.EINVALID, "failed to unmarshal value JSON into object", err)
	}
	if len(schema) == 0 {
		return nil, ez.New(op, ez.EINVALID, "value is an empty JSON object", nil)
	}

	return schema, nil
}

func stringifyResult(r *mcpproto.CallToolResult) string {
	if r == nil {
		return ""
	}
	var b strings.Builder
	for _, c := range r.Content {
		switch v := c.(type) {
		case mcpproto.TextContent: // value
			if v.Text != "" {
				if b.Len() > 0 {
					b.WriteByte('\n')
				}
				b.WriteString(v.Text)
			}
		case *mcpproto.TextContent: // pointer (some servers return pointers)
			if v != nil && v.Text != "" {
				if b.Len() > 0 {
					b.WriteByte('\n')
				}
				b.WriteString(v.Text)
			}
		case mcpproto.EmbeddedResource:
			// If the tool returned an embedded text resource, include it
			switch res := v.Resource.(type) {
			case mcpproto.TextResourceContents:
				if res.Text != "" {
					if b.Len() > 0 {
						b.WriteByte('\n')
					}
					b.WriteString(res.Text)
				}
			case *mcpproto.TextResourceContents:
				if res != nil && res.Text != "" {
					if b.Len() > 0 {
						b.WriteByte('\n')
					}
					b.WriteString(res.Text)
				}
			}
			// case mcpproto.ResourceLink, mcpproto.ImageContent, mcpproto.AudioContent:
			// ignore it for now
		}
	}
	return b.String()
}
