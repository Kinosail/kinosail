package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"
	"time"
)

func TestMCPRecommendationContextReadsOnlyRecentHistoryPage(t *testing.T) {
	issuer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writeJSON(writer, map[string]any{"active": true, "scope": mcpReadScope, "exp": time.Now().Add(time.Hour).Unix(), "sub": "subject", "aud": "http://localhost/mcp"}, http.StatusOK)
	}))
	defer issuer.Close()
	viewer := viewerProfile{ID: "viewer", Name: "Viewer", Libraries: []string{"all"}, Rating: "all", OIDCIssuer: issuer.URL, OIDCSubject: "subject"}
	items := snapshotFixtureItems(14_000)
	fixture := newSnapshotFixture(t, items)
	for _, item := range items {
		if err := fixture.progress.Set(snapshotRequest(t.Context(), http.MethodPut, "/", viewer), item.ID, 30, nil); err != nil {
			t.Fatal(err)
		}
	}
	full := httptest.NewRecorder()
	fixture.mux.ServeHTTP(full, snapshotRequest(t.Context(), http.MethodGet, "/api/v1/history", viewer))
	var history historyPageBody
	if err := json.Unmarshal(full.Body.Bytes(), &history); err != nil || full.Code != http.StatusOK || len(history.Items) != len(items) || full.Body.Len() <= 4<<20 {
		t.Fatalf("full history = %d, %d items, %d bytes, %v; want every item above the 4 MiB MCP response limit", full.Code, len(history.Items), full.Body.Len(), err)
	}

	profiles := newProfileStore("")
	profiles.profiles = []viewerProfile{viewer}
	authentication := &authentication{profiles: profiles, audit: newAuditStore(t.Context(), t.TempDir(), nil)}
	config := MCPConfig{ResourceURL: "http://localhost/mcp", AuthorizationServer: issuer.URL, IntrospectionURL: issuer.URL + "/introspect", ClientID: "resource", ClientSecret: "secret"}
	registerMCPWithConnections(fixture.mux, config, authentication, apiRouting(fixture.mux), nil)
	response := mcpRequest(t, fixture.mux, "tools/call", "access-token", map[string]any{"name": "recommendation_context", "arguments": map[string]any{"view": "movies", "limit": 10}})
	var envelope struct {
		Result struct {
			IsError           bool `json:"isError"`
			StructuredContent struct {
				History struct {
					Status int             `json:"status"`
					Body   historyPageBody `json:"body"`
				} `json:"history"`
				Candidates struct {
					Status int `json:"status"`
				} `json:"candidates"`
			} `json:"structuredContent"`
		} `json:"result"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("recommendation context = %d %.2000q: %v", response.Code, response.Body.String(), err)
	}
	content := envelope.Result.StructuredContent
	want := historyIDs(history)[:10]
	if response.Code != http.StatusOK || envelope.Result.IsError || content.History.Status != http.StatusOK || content.Candidates.Status != http.StatusOK || !slices.Equal(historyIDs(content.History.Body), want) {
		t.Fatalf("recommendation context = %d %.2000q, want history %v", response.Code, response.Body.String(), want)
	}
}
