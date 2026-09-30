package mcpgateway

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/MikeO7/kinosail/packages/httpguard"
	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func limitMCPRequests(limiter *httpguard.Limiter, next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if !limiter.Allow(httpguard.RemoteHost(request.RemoteAddr), 120) {
			writer.Header().Set("Retry-After", "60")
			http.Error(writer, "MCP request limit reached; retry in one minute", http.StatusTooManyRequests)
			return
		}
		next.ServeHTTP(writer, request)
	})
}

func (adapter *Gateway) limitTools(next mcp.MethodHandler) mcp.MethodHandler {
	return func(ctx context.Context, method string, request mcp.Request) (mcp.Result, error) {
		if method == "tools/call" {
			profile, err := adapter.apiPrincipal(ctx)
			if err == nil && !adapter.tools.Allow(profile.ID, 120) {
				err = errors.New("MCP tool limit reached; retry in one minute")
			}
			if err != nil {
				result := &mcp.CallToolResult{}
				result.SetError(err)
				return result, nil
			}
		}
		return next(ctx, method, request)
	}
}

func mcpMethodNotAllowed(writer http.ResponseWriter, _ *http.Request) {
	writer.Header().Set("Allow", http.MethodPost)
	http.Error(writer, "Method Not Allowed", http.StatusMethodNotAllowed)
}

func modernMCPResults(next mcp.MethodHandler) mcp.MethodHandler {
	return func(ctx context.Context, method string, request mcp.Request) (mcp.Result, error) {
		if strings.HasPrefix(method, "prompts/") || strings.HasPrefix(method, "resources/") || method == "completion/complete" {
			return nil, &jsonrpc.Error{Code: jsonrpc.CodeMethodNotFound, Message: "method not available"}
		}
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
			code, message := mcp.CodeUnsupportedProtocolVersion, "unsupported protocol version"
			if request.Header.Get("MCP-Protocol-Version") == "" {
				code, message = mcp.CodeHeaderMismatch, "missing MCP-Protocol-Version header"
			}
			writer.Header().Set("Content-Type", "application/json")
			writer.WriteHeader(http.StatusBadRequest)
			response := map[string]any{
				"jsonrpc": "2.0",
				"error": map[string]any{
					"code":    code,
					"message": message,
					"data":    map[string]any{"requested": request.Header.Get("MCP-Protocol-Version"), "supported": []string{ProtocolVersion}},
				},
			}
			if id := mcpErrorRequestID(request); id != nil {
				response["id"] = id
			}
			_ = json.NewEncoder(writer).Encode(response)
			return
		}
		next.ServeHTTP(writer, request)
	})
}

func mcpErrorRequestID(request *http.Request) any {
	var envelope map[string]json.RawMessage
	if httpguard.DecodeUniqueJSON(request.Body, mcp.DefaultMaxRequestBodyBytes, &envelope) != nil {
		return nil
	}
	decoder := json.NewDecoder(bytes.NewReader(envelope["id"]))
	decoder.UseNumber()
	var value any
	if decoder.Decode(&value) == nil {
		switch id := value.(type) {
		case string:
			return id
		case json.Number:
			if _, err := id.Int64(); err == nil {
				return id
			}
		}
	}
	return nil
}
