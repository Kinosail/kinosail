package server

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail/packages/mcpgateway"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestMCPStdioUsesSoleOwnerAndSharedAPI(t *testing.T) { //nolint:cyclop,gocognit,funlen // One integration lifecycle proves setup, authentication, tools, mutation, and shutdown.
	dataDir := t.TempDir()
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	app := New(Config{Lifecycle: ctx, DataDir: dataDir, RequireAuth: true})
	socketPath, _ := mcpgateway.StdioSocketPath(dataDir)
	if info, err := os.Stat(socketPath); err != nil || info.Mode()&os.ModeSocket == 0 || info.Mode().Perm() != 0o600 {
		t.Fatalf("private MCP socket = %#v, %v", info, err)
	}
	setupRequest := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/v1/setup", strings.NewReader(`{"name":"Owner","password":"owner-password","device":"MCP test","totp":true}`))
	setupRequest.Header.Set("Content-Type", "application/json")
	setup := httptest.NewRecorder()
	app.ServeHTTP(setup, setupRequest)
	var owner struct {
		Token string
		TOTP  struct{ Secret string }
	}
	if err := json.Unmarshal(setup.Body.Bytes(), &owner); err != nil {
		t.Fatal(err)
	}
	if setup.Code != http.StatusCreated || owner.Token == "" {
		t.Fatalf("setup = %d %q", setup.Code, setup.Body.String())
	}
	confirmRequest := httptest.NewRequestWithContext(t.Context(), http.MethodPut, "/api/v1/me/mfa", strings.NewReader(`{"code":"`+routeTestTOTP(owner.TOTP.Secret)+`"}`))
	confirmRequest.Header.Set("Content-Type", "application/json")
	confirmRequest.Header.Set("Authorization", "Bearer "+owner.Token)
	confirm := httptest.NewRecorder()
	app.ServeHTTP(confirm, confirmRequest)
	if confirm.Code != http.StatusOK {
		t.Fatalf("MFA confirmation = %d %q", confirm.Code, confirm.Body.String())
	}

	serverReader, clientWriter := io.Pipe()
	clientReader, serverWriter := io.Pipe()
	errCh := make(chan error, 1)
	go func() {
		errCh <- ServeMCPStdio(ctx, Config{DataDir: dataDir}, "", serverReader, serverWriter)
	}()

	client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "1"}, nil)
	session, err := client.Connect(ctx, &mcp.IOTransport{Reader: clientReader, Writer: clientWriter}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	tools, err := session.ListTools(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]bool{"read_api": false, "write_api": false, "manage_api": false}
	for _, tool := range tools.Tools {
		if _, ok := want[tool.Name]; ok {
			want[tool.Name] = true
		}
	}
	for name, found := range want {
		if !found {
			t.Errorf("Owner STDIO tools omit %q", name)
		}
	}
	result, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "read_api", Arguments: map[string]any{"path": "/api/v1/me"}})
	if err != nil || !bytes.Contains([]byte(result.Content[0].(*mcp.TextContent).Text), []byte(`"name":"Owner"`)) {
		t.Fatalf("read_api = %#v, %v", result, err)
	}
	result, err = session.CallTool(ctx, &mcp.CallToolParams{Name: "create_playlist", Arguments: map[string]any{"name": "From MCP", "ids": []string{}}})
	if err != nil || result.IsError {
		t.Fatalf("create_playlist = %#v, %v", result, err)
	}
	playlistsRequest := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/playlists", nil)
	playlistsRequest.Header.Set("Authorization", "Bearer "+owner.Token)
	playlists := httptest.NewRecorder()
	app.ServeHTTP(playlists, playlistsRequest)
	if playlists.Code != http.StatusOK || !strings.Contains(playlists.Body.String(), `"From MCP"`) {
		t.Fatalf("running Server did not observe MCP mutation: %d %q", playlists.Code, playlists.Body.String())
	}
	audit, err := os.ReadFile(dataDir + "/audit.jsonl")
	if err != nil || !bytes.Contains(audit, []byte(`"action":"playlist.updated"`)) || !bytes.Contains(audit, []byte(`"actor":"Host MCP · Owner"`)) {
		t.Fatalf("MCP audit attribution = %q, %v", audit, err)
	}
	cancel()
	_ = session.Close()
	<-errCh
}

func TestMCPStdioRejectsMissingStreamsBeforeSideEffects(t *testing.T) {
	dataDir := t.TempDir()
	if err := ServeMCPStdio(t.Context(), Config{DataDir: dataDir}, "", nil, io.Discard); err == nil {
		t.Fatal("missing input stream was accepted")
	}
	entries, err := os.ReadDir(dataDir)
	if err != nil || len(entries) != 0 {
		t.Fatalf("rejected STDIO input changed data directory: %v, %v", entries, err)
	}
}

func TestMCPStdioRequiresRunningServerWithoutSideEffects(t *testing.T) {
	dataDir := t.TempDir()
	if err := ServeMCPStdio(t.Context(), Config{DataDir: dataDir}, "", strings.NewReader(""), io.Discard); err == nil || !strings.Contains(err.Error(), "running Kinosail Server") {
		t.Fatalf("absent Server = %v", err)
	}
	entries, err := os.ReadDir(dataDir)
	if err != nil || len(entries) != 0 {
		t.Fatalf("absent Server bridge changed data directory: %v, %v", entries, err)
	}
}
