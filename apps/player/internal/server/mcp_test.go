package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestMCPModernOAuthAndAPIDrivenTools(t *testing.T) { //nolint:cyclop,funlen,gocognit // One fixture proves the complete MCP boundary.
	scope, audience := mcpReadScope, "http://localhost/mcp"
	libraryCalls, playlistCalls := 0, 0
	issuer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost || request.URL.Path != "/introspect" {
			http.NotFound(writer, request)
			return
		}
		if id, secret, ok := request.BasicAuth(); !ok || id != "resource" || secret != "secret" {
			t.Fatal("introspection client authentication was not forwarded")
		}
		_ = request.ParseForm()
		if request.Form.Get("token") != "access-token" || request.Form.Get("token_type_hint") != "access_token" {
			t.Fatalf("introspection form = %#v", request.Form)
		}
		writeJSON(writer, map[string]any{"active": true, "scope": scope, "exp": time.Now().Add(time.Hour).Unix(), "sub": "subject", "aud": audience}, http.StatusOK)
	}))
	defer issuer.Close()

	profiles := newProfileStore("")
	profiles.profiles = []viewerProfile{{ID: "viewer", Name: "Viewer", Libraries: []string{"all"}, Rating: "all", OIDCIssuer: issuer.URL, OIDCSubject: "subject"}}
	authentication := &authentication{profiles: profiles, audit: newAuditStore(t.Context(), t.TempDir(), nil)}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/me", func(writer http.ResponseWriter, request *http.Request) {
		writeJSON(writer, map[string]string{"viewer": currentViewer(request).Name}, http.StatusOK)
	})
	mux.HandleFunc("GET /api/v1/library", func(writer http.ResponseWriter, request *http.Request) {
		libraryCalls++
		if request.URL.Query().Get("q") == "oversized-response" {
			writeJSON(writer, map[string]string{"value": strings.Repeat("x", 5<<20)}, http.StatusOK)
			return
		}
		writeJSON(writer, map[string]any{"items": []any{map[string]string{"id": "movie", "title": request.URL.Query().Get("q")}}}, http.StatusOK)
	})
	mux.HandleFunc("GET /api/v1/history", func(writer http.ResponseWriter, _ *http.Request) {
		writeJSON(writer, map[string]any{"items": []any{map[string]string{"id": "watched"}}}, http.StatusOK)
	})
	mux.HandleFunc("GET /api/v1/metrics", func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "text/plain")
		_, _ = writer.Write([]byte("kinosail_health 1\n"))
	})
	mux.HandleFunc("POST /api/v1/playlists", func(writer http.ResponseWriter, request *http.Request) {
		playlistCalls++
		var input map[string]any
		_ = json.NewDecoder(request.Body).Decode(&input)
		writeJSON(writer, input, http.StatusCreated)
	})
	mux.HandleFunc("PUT /api/v1/me/language", func(writer http.ResponseWriter, request *http.Request) {
		var input map[string]string
		_ = json.NewDecoder(request.Body).Decode(&input)
		writeJSON(writer, input, http.StatusOK)
	})
	mux.Handle("GET /api/v1/settings", authentication.owner(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writeJSON(writer, map[string]string{"setting": "private"}, http.StatusOK)
	})))
	mux.Handle("PUT /api/v1/settings/server", authentication.owner(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		var input map[string]string
		_ = json.NewDecoder(request.Body).Decode(&input)
		writeJSON(writer, input, http.StatusOK)
	})))
	mux.Handle("PUT /api/v1/profiles/{id}/password", authentication.owner(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writeJSON(writer, map[string]string{"status": "changed"}, http.StatusOK)
	})))
	config := MCPConfig{ResourceURL: audience, AuthorizationServer: issuer.URL, IntrospectionURL: issuer.URL + "/introspect", ClientID: "resource", ClientSecret: "secret"}
	registerMCPWithConnections(mux, config, authentication, apiRouting(mux), nil)

	t.Run("metadata", func(t *testing.T) {
		response := httptest.NewRecorder()
		mux.ServeHTTP(response, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/.well-known/oauth-protected-resource/mcp", nil))
		if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), issuer.URL) || !strings.Contains(response.Body.String(), mcpReadScope) || !strings.Contains(response.Body.String(), mcpWriteScope) || !strings.Contains(response.Body.String(), mcpManageScope) {
			t.Fatalf("metadata = %d %q", response.Code, response.Body.String())
		}
	})

	t.Run("stateless methods", func(t *testing.T) {
		for _, method := range []string{http.MethodGet, http.MethodDelete} {
			response := httptest.NewRecorder()
			mux.ServeHTTP(response, httptest.NewRequestWithContext(t.Context(), method, "/mcp", nil))
			if response.Code != http.StatusMethodNotAllowed || response.Header().Get("Allow") != http.MethodPost {
				t.Fatalf("%s /mcp = %d allow=%q", method, response.Code, response.Header().Get("Allow"))
			}
		}
	})

	t.Run("oauth challenge", func(t *testing.T) {
		missingVersion := httptest.NewRecorder()
		mux.ServeHTTP(missingVersion, httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/mcp", strings.NewReader("{}")))
		if missingVersion.Code != http.StatusUnauthorized || !strings.Contains(missingVersion.Header().Get("WWW-Authenticate"), "resource_metadata=\"http://localhost/.well-known/oauth-protected-resource/mcp\"") {
			t.Fatalf("versionless challenge = %d %q %q", missingVersion.Code, missingVersion.Header().Get("WWW-Authenticate"), missingVersion.Body.String())
		}
		response := mcpRequest(t, mux, "server/discover", "", nil)
		if response.Code != http.StatusUnauthorized || !strings.Contains(response.Header().Get("WWW-Authenticate"), "resource_metadata=\"http://localhost/.well-known/oauth-protected-resource/mcp\"") {
			t.Fatalf("challenge = %d %q %q", response.Code, response.Header().Get("WWW-Authenticate"), response.Body.String())
		}
	})

	t.Run("modern discovery", func(t *testing.T) {
		response := mcpRequest(t, mux, "server/discover", "access-token", nil)
		if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"supportedVersions":["2026-07-28"]`) || strings.Contains(response.Body.String(), "2025-") || !strings.Contains(response.Body.String(), `"cacheScope":"private"`) {
			t.Fatalf("discover = %d %q", response.Code, response.Body.String())
		}
	})

	t.Run("read-only tool list", func(t *testing.T) {
		response := mcpRequest(t, mux, "tools/list", "access-token", nil)
		if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"name":"read_api"`) || !strings.Contains(response.Body.String(), `"name":"search_media"`) || !strings.Contains(response.Body.String(), `"name":"recommendation_context"`) || strings.Contains(response.Body.String(), `"name":"write_api"`) || strings.Contains(response.Body.String(), `"name":"manage_api"`) || !strings.Contains(response.Body.String(), `"readOnlyHint":true`) {
			t.Fatalf("tools = %d %q", response.Code, response.Body.String())
		}
	})

	t.Run("agent media tools", func(t *testing.T) {
		response := mcpRequest(t, mux, "tools/call", "access-token", map[string]any{"name": "search_media", "arguments": map[string]any{"query": "Halloween", "view": "movies", "limit": 10}})
		if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"title":"Halloween"`) {
			t.Fatalf("search = %d %q", response.Code, response.Body.String())
		}
		response = mcpRequest(t, mux, "tools/call", "access-token", map[string]any{"name": "recommendation_context", "arguments": map[string]any{"view": "shows", "limit": 10}})
		if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"history"`) || !strings.Contains(response.Body.String(), `"candidates"`) || !strings.Contains(response.Body.String(), `"id":"watched"`) {
			t.Fatalf("recommendation context = %d %q", response.Code, response.Body.String())
		}
	})

	t.Run("read API", func(t *testing.T) {
		response := mcpRequest(t, mux, "tools/call", "access-token", map[string]any{"name": "read_api", "arguments": map[string]string{"path": "/api/v1/me"}})
		if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"viewer":"Viewer"`) || !strings.Contains(response.Body.String(), `"status":200`) {
			t.Fatalf("read = %d %q", response.Code, response.Body.String())
		}
	})

	t.Run("monitoring metrics", func(t *testing.T) {
		profiles.profiles[0].Owner = true
		profiles.profiles[0].TOTPSecret = "fixture-owner-factor"
		response := mcpRequest(t, mux, "tools/call", "access-token", map[string]any{"name": "read_api", "arguments": map[string]string{"path": "/api/v1/metrics"}})
		profiles.profiles[0].Owner = false
		profiles.profiles[0].TOTPSecret = ""
		if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "API operation is not available through MCP") {
			t.Fatalf("read scope reached Owner metrics = %d %q", response.Code, response.Body.String())
		}
	})

	t.Run("Viewer role policy", func(t *testing.T) {
		response := mcpRequest(t, mux, "tools/call", "access-token", map[string]any{"name": "read_api", "arguments": map[string]string{"path": "/api/v1/settings"}})
		if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "API operation is not available through MCP") {
			t.Fatalf("read scope reached Owner API = %d %q", response.Code, response.Body.String())
		}
	})

	t.Run("write scope", func(t *testing.T) {
		scope = mcpReadScope + " " + mcpWriteScope
		response := mcpRequest(t, mux, "tools/list", "access-token", nil)
		if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"name":"write_api"`) || !strings.Contains(response.Body.String(), `"name":"create_playlist"`) {
			t.Fatalf("tools = %d %q", response.Code, response.Body.String())
		}
		response = mcpRequest(t, mux, "tools/call", "access-token", map[string]any{"name": "write_api", "arguments": map[string]any{"method": "PUT", "path": "/api/v1/me/language", "body": map[string]string{"language": "de"}}})
		if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"language":"de"`) {
			t.Fatalf("write = %d %q", response.Code, response.Body.String())
		}
		response = mcpRequest(t, mux, "tools/call", "access-token", map[string]any{"name": "create_playlist", "arguments": map[string]any{"name": "Halloween", "ids": []string{"movie"}}})
		if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"name":"Halloween"`) || !strings.Contains(response.Body.String(), `"movie"`) {
			t.Fatalf("playlist = %d %q", response.Code, response.Body.String())
		}
		attributed := false
		for _, event := range authentication.audit.Query("", 10) {
			attributed = attributed || event.Action == "playlist.updated" && event.Actor == "MCP · Viewer"
		}
		if !attributed {
			t.Fatalf("OAuth MCP mutation was not attributed: %#v", authentication.audit.Query("", 10))
		}
	})

	t.Run("Owner management scope", func(t *testing.T) {
		scope = mcpReadScope + " " + mcpManageScope
		response := mcpRequest(t, mux, "tools/list", "access-token", nil)
		if response.Code != http.StatusOK || strings.Contains(response.Body.String(), `"name":"manage_api"`) {
			t.Fatalf("Viewer management tools = %d %q", response.Code, response.Body.String())
		}
		profiles.profiles[0].Owner = true
		profiles.profiles[0].TOTPSecret = "fixture-owner-factor"
		response = mcpRequest(t, mux, "tools/list", "access-token", nil)
		if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"name":"manage_api"`) || strings.Contains(response.Body.String(), `"name":"create_playlist"`) {
			t.Fatalf("Owner management tools = %d %q", response.Code, response.Body.String())
		}
		response = mcpRequest(t, mux, "tools/call", "access-token", map[string]any{"name": "manage_api", "arguments": map[string]any{"method": "PUT", "path": "/api/v1/settings/server", "body": map[string]string{"name": "Cinema"}}})
		if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"name":"Cinema"`) {
			t.Fatalf("manage = %d %q", response.Code, response.Body.String())
		}
		response = mcpRequest(t, mux, "tools/call", "access-token", map[string]any{"name": "manage_api", "arguments": map[string]any{"method": "GET", "path": "/api/v1/settings"}})
		if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"setting":"private"`) {
			t.Fatalf("manage read = %d %q", response.Code, response.Body.String())
		}
		response = mcpRequest(t, mux, "tools/call", "access-token", map[string]any{"name": "manage_api", "arguments": map[string]any{"method": "GET", "path": "/api/v1/metrics"}})
		if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "kinosail_health 1") {
			t.Fatalf("manage metrics = %d %q", response.Code, response.Body.String())
		}
		response = mcpRequest(t, mux, "tools/call", "access-token", map[string]any{"name": "manage_api", "arguments": map[string]any{"method": "PUT", "path": "/api/v1/profiles/viewer/password", "body": map[string]string{"password": "unsafe"}}})
		if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "API operation is not available through MCP") {
			t.Fatalf("identity management = %d %q", response.Code, response.Body.String())
		}
	})

	t.Run("audience binding", func(t *testing.T) {
		audience = "http://other.test/mcp"
		response := mcpRequest(t, mux, "tools/list", "access-token", nil)
		if response.Code != http.StatusUnauthorized {
			t.Fatalf("foreign audience = %d %q", response.Code, response.Body.String())
		}
	})

	t.Run("origin validation", func(t *testing.T) {
		audience = "http://localhost/mcp"
		request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/mcp", strings.NewReader("{}"))
		request.Header.Set("MCP-Protocol-Version", mcpProtocolVersion)
		request.Header.Set("Origin", "https://attacker.example")
		response := httptest.NewRecorder()
		mux.ServeHTTP(response, request)
		if response.Code != http.StatusForbidden {
			t.Fatalf("cross-origin MCP = %d %q", response.Code, response.Body.String())
		}
	})

	t.Run("bounded adapter input and output", func(t *testing.T) {
		scope = mcpReadScope + " " + mcpWriteScope
		profiles.profiles[0].Owner = false
		profiles.profiles[0].TOTPSecret = ""
		beforeReads := libraryCalls
		response := mcpRequest(t, mux, "tools/call", "access-token", map[string]any{"name": "write_api", "arguments": map[string]any{"method": "GET", "path": "/api/v1/library"}})
		if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "method is not available") || libraryCalls != beforeReads {
			t.Fatalf("write method confusion = %d %q reads=%d", response.Code, response.Body.String(), libraryCalls-beforeReads)
		}
		response = mcpRequest(t, mux, "tools/call", "access-token", map[string]any{"name": "read_api", "arguments": map[string]string{"path": "/api/v1/library?q=" + strings.Repeat("x", 2048)}})
		if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "API path exceeds") || libraryCalls != beforeReads {
			t.Fatalf("oversized path = %d %q reads=%d", response.Code, response.Body.String(), libraryCalls-beforeReads)
		}
		beforeWrites := playlistCalls
		response = mcpRequest(t, mux, "tools/call", "access-token", map[string]any{"name": "write_api", "arguments": map[string]any{"method": "POST", "path": "/api/v1/playlists", "body": map[string]any{"name": strings.Repeat("x", (1<<20)+1), "ids": []string{}}}})
		if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "API request body exceeds") || playlistCalls != beforeWrites {
			t.Fatalf("oversized body = %d %q writes=%d", response.Code, response.Body.String(), playlistCalls-beforeWrites)
		}
		response = mcpRequest(t, mux, "tools/call", "access-token", map[string]any{"name": "read_api", "arguments": map[string]string{"path": "/api/v1/library?q=oversized-response"}})
		if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "API response exceeds") || strings.Contains(response.Body.String(), strings.Repeat("x", 1024)) {
			t.Fatalf("oversized response = %d bytes=%d %q", response.Code, response.Body.Len(), response.Body.String())
		}
	})
}
