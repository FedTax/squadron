package aitools

import (
	"context"
	"encoding/json"
	"strings"
)

// GatewayLinkTool associates the calling run with a gateway request key
// so subsequent questions, notifications, and posts share the same request.
type GatewayLinkTool struct {
	Bridge GatewayBridge
}

func (t *GatewayLinkTool) ToolName() string { return "link" }

func (t *GatewayLinkTool) ToolDescription() string {
	return "Join this run to the request the gateway already holds under `key` (the request key runs pass to the gateway, e.g. a ticket key), " +
		"so this run's questions, notifications and posts go to that request. Call it before asking or posting anything. " +
		"If no gateway is configured, returns \"[no gateway configured]\" so you can proceed without failing."
}

func (t *GatewayLinkTool) ToolPayloadSchema() Schema {
	return Schema{
		Type: TypeObject,
		Properties: PropertyMap{
			"key": {
				Type:        TypeString,
				Description: "The request key, e.g. a ticket key.",
			},
		},
		Required: []string{"key"},
	}
}

func (t *GatewayLinkTool) Call(ctx context.Context, params string) string {
	if t.Bridge == nil {
		return NoGatewayObservation
	}
	var p struct {
		Key string `json:"key"`
	}
	if err := json.Unmarshal([]byte(params), &p); err != nil {
		return "Error: invalid JSON parameters"
	}
	key := strings.TrimSpace(p.Key)
	if key == "" {
		return "Error: key is required"
	}
	if err := t.Bridge.Link(ctx, key); err != nil {
		return "Error: " + err.Error()
	}
	return "Run linked to the request for " + key + "."
}
