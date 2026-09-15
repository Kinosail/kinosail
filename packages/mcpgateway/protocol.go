package mcpgateway

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func mcpMethodNotAllowed(writer http.ResponseWriter, _ *http.Request) {
	writer.Header().Set("Allow", http.MethodPost)
	http.Error(writer, "Method Not Allowed", http.StatusMethodNotAllowed)
}

func modernMCPResults(next mcp.MethodHandler) mcp.MethodHandler {
	return func(ctx context.Context, method string, request mcp.Request) (mcp.Result, error) {
		result, err := next(ctx, method, request)
		if err != nil {
			return nil, err
		}
		switch value := result.(type) {
		case *mcp.DiscoverResult:
			value.SupportedVersions = []string{ProtocolVersion}
			value.CacheScope = "private"
		case *mcp.ListToolsResult:
			value.CacheScope = "private"
		}
		return result, nil
	}
}

func modernMCP(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("MCP-Protocol-Version") != ProtocolVersion {
			writer.Header().Set("Content-Type", "application/json")
			writer.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(writer).Encode(map[string]any{
				"jsonrpc": "2.0",
				"id":      nil,
				"error": map[string]any{
					"code":    mcp.CodeUnsupportedProtocolVersion,
					"message": "unsupported protocol version",
					"data":    map[string]any{"requested": request.Header.Get("MCP-Protocol-Version"), "supported": []string{ProtocolVersion}},
				},
			})
			return
		}
		next.ServeHTTP(writer, request)
	})
}
