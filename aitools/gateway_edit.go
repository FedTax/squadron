package aitools

import (
	"context"
	"encoding/json"
	"strings"
)

// GatewayEditTool backs builtins.gateway.edit. A run that reports progress in
// one message (a status line, a checklist) can change that message instead
// of posting a new one each time, so the conversation shows the current
// state rather than every step. The replacement uses the gateway's own post
// schema, so whatever a post can say, an edit can say.
type GatewayEditTool struct {
	Bridge GatewayBridge
}

func (t *GatewayEditTool) ToolName() string { return "edit" }

const gatewayEditIDProperty = `{"type": "string", "description": "The id builtins.gateway.post returned for the message to change."}`

func (t *GatewayEditTool) ToolDescription() string {
	return "Replace the content of a message posted earlier with builtins.gateway.post. " +
		"`id` is the id that post returned; the other fields are the new content, shaped as for builtins.gateway.post, and replace the whole message. " +
		"If no gateway is configured, returns \"[no gateway configured]\" so you can proceed without failing."
}

func (t *GatewayEditTool) ToolPayloadSchema() Schema {
	raw := ""
	if t.Bridge != nil {
		raw = t.Bridge.MessageToolSchema()
	}
	if strings.TrimSpace(raw) == "" {
		raw = defaultGatewayPostSchema
	}
	raw = injectProperty(raw, "id", gatewayEditIDProperty, true)
	return Schema{Type: TypeObject, Properties: PropertyMap{}}.WithRawJSONSchema(json.RawMessage(raw))
}

func (t *GatewayEditTool) Call(ctx context.Context, params string) string {
	if t.Bridge == nil {
		return NoGatewayObservation
	}
	var root map[string]json.RawMessage
	if err := json.Unmarshal([]byte(params), &root); err != nil {
		return "Error: invalid JSON parameters"
	}
	var id string
	if raw, ok := root["id"]; ok {
		if err := json.Unmarshal(raw, &id); err != nil {
			return "Error: id must be a string"
		}
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return "Error: id is required"
	}
	delete(root, "id")
	if len(root) == 0 {
		return "Error: empty message payload"
	}
	payload, err := json.Marshal(root)
	if err != nil {
		return "Error: " + err.Error()
	}
	if err := t.Bridge.EditMessage(ctx, id, string(payload)); err != nil {
		return "Error: " + err.Error()
	}
	return "Message " + id + " updated."
}
