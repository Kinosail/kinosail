package mcpgateway

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"log/slog"

	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
)

func logEventRequestFailure(operation string, err error) {
	if err == nil {
		return
	}
	switch operation {
	case "events/list", "events/subscribe", "events/unsubscribe":
	default:
		return
	}
	failure := "request_failed"
	var rpc *jsonrpc.Error
	if errors.As(err, &rpc) {
		if category, known := (map[int64]string{-32602: "invalid_input", -32011: "not_found", -32012: "access_unavailable", -32013: "resource_unavailable", -32014: "unsupported", -32015: "callback_failed"})[rpc.Code]; known {
			failure = category
		}
		var data struct {
			Reason string `json:"reason"`
		}
		if json.Unmarshal(rpc.Data, &data) == nil {
			switch data.Reason {
			case "timeout", "tls_error", "connection_refused", "http_4xx", "http_5xx", "challenge_failed":
				failure = data.Reason
			}
		}
	}
	slog.Warn("MCP event request failed", "operation", operation, "request_id", rand.Text(), "failure", failure)
}
