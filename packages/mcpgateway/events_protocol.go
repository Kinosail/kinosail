package mcpgateway

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"

	"github.com/MikeO7/kinosail/packages/httpguard"
	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type eventParams struct {
	mcp.ParamsBase
	Name      string            `json:"name"`
	Arguments map[string]string `json:"arguments"`
	Delivery  eventDestination  `json:"delivery"`
	Cursor    *string           `json:"cursor,omitempty"`
	TTL       json.RawMessage   `json:"ttlMs,omitempty"`
	MaxAge    json.RawMessage   `json:"maxAgeMs,omitempty"`
}

type eventDestination struct {
	Mode   string `json:"mode"`
	URL    string `json:"url"`
	Secret string `json:"secret,omitempty"`
}

func (params *eventParams) UnmarshalJSON(data []byte) error {
	type plain eventParams
	return httpguard.DecodeUniqueJSON(bytes.NewReader(data), 16<<10, (*plain)(params))
}

type eventListParams struct {
	mcp.ParamsBase
	Cursor string `json:"cursor,omitempty"`
}

func (params *eventListParams) UnmarshalJSON(data []byte) error {
	type plain eventListParams
	return httpguard.DecodeUniqueJSON(bytes.NewReader(data), 4<<10, (*plain)(params))
}

type eventListResult struct {
	mcp.ResultBase
	Events []eventDefinition `json:"events"`
}

type eventResult struct {
	mcp.ResultBase
	ID            string  `json:"id"`
	RefreshBefore string  `json:"refreshBefore"`
	Cursor        *string `json:"cursor"`
	Truncated     bool    `json:"truncated"`
}

type eventDefinition struct {
	Name          string         `json:"name"`
	Description   string         `json:"description"`
	Delivery      []string       `json:"delivery"`
	InputSchema   map[string]any `json:"inputSchema"`
	PayloadSchema map[string]any `json:"payloadSchema"`
}

type eventDiscoverResult struct {
	*mcp.DiscoverResult
}

func (result *eventDiscoverResult) MarshalJSON() ([]byte, error) {
	data, err := json.Marshal(result.DiscoverResult)
	if err != nil {
		return nil, err
	}
	var value map[string]any
	if err = json.Unmarshal(data, &value); err != nil {
		return nil, err
	}
	value["capabilities"].(map[string]any)["events"] = map[string]any{}
	return json.Marshal(value)
}

func eventError(code int64, message string, data any) error {
	encoded, _ := json.Marshal(data)
	return &jsonrpc.Error{Code: code, Message: message, Data: encoded}
}

func (adapter *Gateway) registerEvents(server *mcp.Server) { //nolint:gocognit // Capability wrapping and access/rate checks share the protocol boundary.
	// These names belong to the draft extension and cannot shadow SDK methods.
	_ = mcp.AddReceivingCustomMethod(server, "events/list", adapter.listEvents)
	_ = mcp.AddReceivingCustomMethod(server, "events/subscribe", adapter.subscribeEvent)
	_ = mcp.AddReceivingCustomMethod(server, "events/unsubscribe", adapter.unsubscribeEvent)
	server.AddReceivingMiddleware(func(next mcp.MethodHandler) mcp.MethodHandler {
		return func(ctx context.Context, method string, request mcp.Request) (result mcp.Result, err error) {
			defer func() { logEventRequestFailure(method, err) }()
			if strings.HasPrefix(method, "events/") && adapter.connections == nil {
				return nil, eventError(-32014, "event persistence is unavailable", map[string]string{"feature": "events"})
			}
			if strings.HasPrefix(method, "events/") {
				profile, err := adapter.apiPrincipal(ctx)
				if err != nil {
					return nil, eventError(-32012, "event access is unavailable", nil)
				}
				if !adapter.tools.Allow(profile.ID, 120) {
					return nil, eventError(-32013, "event request limit reached", map[string]string{"limit": "requests"})
				}
			}
			result, err = next(ctx, method, request)
			if discovered, ok := result.(*mcp.DiscoverResult); ok && adapter.connections != nil && err == nil {
				return &eventDiscoverResult{discovered}, nil
			}
			return result, err
		}
	})
}

func (adapter *Gateway) listEvents(ctx context.Context, _ *mcp.ServerSession, params *eventListParams) (*eventListResult, error) {
	if params != nil && params.Cursor != "" {
		return nil, eventError(-32602, "event cursor is not valid", nil)
	}
	profile, err := adapter.apiPrincipal(ctx)
	if err != nil {
		return nil, eventError(-32012, "event access is unavailable", nil)
	}
	names := []string{"library.updated", "download.updated", "home-assistant.command"}
	if adapter.subtitleEvents && profile.Owner && adapter.eventManage(ctx) {
		names = append(names, "subtitles.updated")
	}
	result := &eventListResult{Events: make([]eventDefinition, 0, len(names))}
	for _, name := range names {
		description, _, _ := eventKind(name)
		result.Events = append(result.Events, eventDefinition{Name: name, Description: description, Delivery: []string{"webhook"}, InputSchema: map[string]any{"type": "object", "properties": map[string]any{"resource": map[string]any{"type": "string", "maxLength": 2048, "description": "Optional exact API resource path to monitor."}}, "additionalProperties": false}, PayloadSchema: map[string]any{"type": "object", "properties": map[string]any{"resource": map[string]string{"type": "string"}}, "required": []string{"resource"}, "additionalProperties": false}})
	}
	return result, nil
}
