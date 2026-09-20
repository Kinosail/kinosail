package mcpgateway

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func assertGatewayProtocol(t *testing.T, mux *http.ServeMux) { //nolint:cyclop // The score of 12 remains below the repository ceiling of 22 for the protocol assertion matrix.
	t.Helper()
	metadata := httptest.NewRecorder()
	mux.ServeHTTP(metadata, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/.well-known/oauth-protected-resource/mcp", nil))
	if metadata.Code != http.StatusOK || !strings.Contains(metadata.Body.String(), ManageScope) {
		t.Fatalf("metadata = %d %q", metadata.Code, metadata.Body.String())
	}
	for _, method := range []string{http.MethodGet, http.MethodDelete} {
		response := httptest.NewRecorder()
		mux.ServeHTTP(response, httptest.NewRequestWithContext(t.Context(), method, "/mcp", nil))
		if response.Code != http.StatusMethodNotAllowed || response.Header().Get("Allow") != http.MethodPost {
			t.Fatalf("%s /mcp = %d", method, response.Code)
		}
	}
	if response := mcpRequest(t, mux, "server/discover", nil); response.Code != http.StatusOK || !strings.Contains(response.Body.String(), ProtocolVersion) || !strings.Contains(response.Body.String(), `"cacheScope":"private"`) {
		t.Fatalf("discover = %d %q", response.Code, response.Body.String())
	}
	if response := mcpRequest(t, mux, "tools/list", nil); response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"name":"read_api"`) || strings.Contains(response.Body.String(), `"name":"write_api"`) {
		t.Fatalf("read tools = %d %q", response.Code, response.Body.String())
	}
	if response := mcpRequest(t, mux, "tools/list", nil); !strings.Contains(response.Body.String(), "untrusted application or media data") {
		t.Fatalf("MCP trust-boundary instructions missing: %d %q", response.Code, response.Body.String())
	}
}

func assertReadTools(t *testing.T, mux *http.ServeMux, api *testAPI) int {
	t.Helper()
	if response := mcpRequest(t, mux, "tools/call", map[string]any{"name": "search_media", "arguments": map[string]any{"query": "Halloween", "limit": 1}}); response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "Halloween") {
		t.Fatalf("search = %d %q", response.Code, response.Body.String())
	}
	if response := mcpRequest(t, mux, "tools/call", map[string]any{"name": "recommendation_context", "arguments": map[string]any{"limit": 1}}); response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "watched") {
		t.Fatalf("recommendations = %d %q", response.Code, response.Body.String())
	}
	before := api.calls
	if response := mcpRequest(t, mux, "tools/call", map[string]any{"name": "read_api", "arguments": map[string]string{"path": "/api/v1/settings"}}); !strings.Contains(response.Body.String(), "not available") || api.calls != before {
		t.Fatalf("blocked route = %d %q calls=%d", response.Code, response.Body.String(), api.calls-before)
	}
	if response := mcpRequest(t, mux, "tools/call", map[string]any{"name": "read_api", "arguments": map[string]string{"path": "/api/v1/library?q=" + strings.Repeat("x", 2048)}}); !strings.Contains(response.Body.String(), "exceeds") || api.calls != before {
		t.Fatalf("oversized path = %q calls=%d", response.Body.String(), api.calls-before)
	}
	return before
}

func assertWriteTools(t *testing.T, mux *http.ServeMux, api *testAPI, viewer Principal, before int) {
	t.Helper()
	if response := mcpRequest(t, mux, "tools/call", map[string]any{"name": "write_api", "arguments": map[string]any{"method": "POST", "path": "/api/v1/playlists", "body": map[string]string{"name": strings.Repeat("x", (1<<20)+1)}}}); !strings.Contains(response.Body.String(), "exceeds") || api.calls != before {
		t.Fatalf("oversized body = %q calls=%d", response.Body.String(), api.calls-before)
	}
	if response := mcpRequest(t, mux, "tools/call", map[string]any{"name": "write_api", "arguments": map[string]any{"method": "GET", "path": "/api/v1/library"}}); !strings.Contains(response.Body.String(), "method is not available") || api.calls != before {
		t.Fatalf("unknown write method = %q calls=%d", response.Body.String(), api.calls-before)
	}
	if response := mcpRequest(t, mux, "tools/call", map[string]any{"name": "write_api", "arguments": map[string]any{"method": "POST", "path": "/api/v1/playlists", "body": map[string]string{"name": "Halloween"}}}); response.Code != http.StatusOK || api.actor != "MCP · Viewer" || api.last != viewer {
		t.Fatalf("write = %d %q actor=%q", response.Code, response.Body.String(), api.actor)
	}
	if response := mcpRequest(t, mux, "tools/call", map[string]any{"name": "create_playlist", "arguments": map[string]any{"name": "Halloween", "ids": []string{"movie"}}}); response.Code != http.StatusOK {
		t.Fatalf("playlist = %d %q", response.Code, response.Body.String())
	}
}

func assertManageTools(t *testing.T, mux *http.ServeMux, api *testAPI) {
	t.Helper()
	if response := mcpRequest(t, mux, "tools/call", map[string]any{"name": "manage_api", "arguments": map[string]any{"method": "GET", "path": "/api/v1/metrics"}}); response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "kinosail_health") || !api.manage {
		t.Fatalf("manage = %d %q", response.Code, response.Body.String())
	}
	if response := mcpRequest(t, mux, "tools/call", map[string]any{"name": "manage_api", "arguments": map[string]any{"method": "GET", "path": "/api/v1/metrics", "body": map[string]bool{"bad": true}}}); !strings.Contains(response.Body.String(), "must not include") {
		t.Fatalf("manage GET body = %q", response.Body.String())
	}
	if response := mcpRequest(t, mux, "tools/call", map[string]any{"name": "read_api", "arguments": map[string]string{"path": "/api/v1/library?q=oversized-response"}}); !strings.Contains(response.Body.String(), "response exceeds") || strings.Contains(response.Body.String(), strings.Repeat("x", 1024)) {
		t.Fatalf("oversized response = %d bytes=%d", response.Code, response.Body.Len())
	}
}
