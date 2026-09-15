package server

import (
	"context"
	"errors"
	"io"
	"slices"
	"testing"
	"time"

	"github.com/MikeO7/kinosail-dashboard/internal/dashboard"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type noCloseReader struct{ io.Reader }

func (noCloseReader) Close() error { return nil }

type noCloseWriter struct{ io.Writer }

func (noCloseWriter) Close() error { return nil }

func runMCPStdio(ctx context.Context, board *dashboard.Service, prober *dashboard.Prober, input io.Reader, output io.Writer) error {
	if ctx == nil || board == nil || prober == nil || input == nil || output == nil {
		return errors.New("MCP STDIO requires application state and streams")
	}
	return newMCPServer(board, prober).Run(ctx, &mcp.IOTransport{Reader: noCloseReader{Reader: input}, Writer: noCloseWriter{Writer: output}})
}

func TestMCPStdioInitializeListAndCall(t *testing.T) {
	app := newTestApplication(t, Config{})
	serverInput, clientOutput := io.Pipe()
	clientInput, serverOutput := io.Pipe()
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	serverDone := make(chan error, 1)
	go func() {
		serverDone <- runMCPStdio(ctx, app.board, app.prober, serverInput, serverOutput)
	}()

	client := mcp.NewClient(&mcp.Implementation{Name: "Kinosail Dashboard contract test", Version: "1"}, nil)
	session, err := client.Connect(ctx, &mcp.IOTransport{Reader: clientInput, Writer: clientOutput}, nil)
	if err != nil {
		t.Fatal(err)
	}

	assertMCPToolInventory(t, ctx, session)
	assertMCPBoardMutationContract(t, ctx, session, app)
	if err := session.Close(); err != nil {
		t.Errorf("close MCP client: %v", err)
	}
	select {
	case err := <-serverDone:
		if err != nil && !errors.Is(err, context.Canceled) && !errors.Is(err, io.EOF) {
			t.Errorf("MCP server exit: %v", err)
		}
	case <-time.After(time.Second):
		cancel()
	}
}

func assertMCPToolInventory(t *testing.T, ctx context.Context, session *mcp.ClientSession) {
	t.Helper()
	listed, err := session.ListTools(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	names := make([]string, 0, len(listed.Tools))
	for _, tool := range listed.Tools {
		names = append(names, tool.Name)
	}
	for _, required := range []string{"list_applications", "list_application_catalog", "add_application", "update_application", "remove_application", "reorder_applications", "check_application"} {
		if !slices.Contains(names, required) {
			t.Errorf("MCP omitted tool %q; tools = %v", required, names)
		}
	}
}

func assertMCPBoardMutationContract(t *testing.T, ctx context.Context, session *mcp.ClientSession, app testApplication) {
	t.Helper()
	assertMCPInitialBoard(t, ctx, session)
	added, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name: "add_application",
		Arguments: map[string]any{
			"name":            "Router",
			"url":             "http://router.lan",
			"checkEnabled":    false,
			"expectedVersion": 1,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if added.IsError {
		t.Fatalf("add application failed: %s", toolText(added.Content))
	}
	if got := app.board.Snapshot(); got.Version != 2 || len(got.Apps) != 1 || got.Apps[0].Name != "Router" {
		t.Fatalf("board after MCP add = %#v", got)
	}

	removed, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name: "remove_application",
		Arguments: map[string]any{
			"id":              app.board.Snapshot().Apps[0].ID,
			"expectedVersion": 2,
			"confirm":         false,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !removed.IsError {
		t.Fatal("MCP removal without confirmation succeeded")
	}
	if got := len(app.board.Snapshot().Apps); got != 1 {
		t.Fatalf("unconfirmed MCP removal left %d applications", got)
	}
}

func assertMCPInitialBoard(t *testing.T, ctx context.Context, session *mcp.ClientSession) {
	t.Helper()
	initial, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "list_applications", Arguments: map[string]any{}})
	if err != nil {
		t.Fatal(err)
	}
	if initial.IsError {
		t.Fatalf("initial list failed: %#v", initial.Content)
	}
	initialBoard := structuredBoard(t, initial.StructuredContent)
	if initialBoard.Version != 1 || len(initialBoard.Apps) != 0 {
		t.Fatalf("initial board = %#v", initialBoard)
	}
}

func toolText(content []mcp.Content) string {
	for _, item := range content {
		if text, ok := item.(*mcp.TextContent); ok {
			return text.Text
		}
	}
	return "no text content"
}

func structuredBoard(t *testing.T, structured any) dashboard.Snapshot {
	t.Helper()
	root, ok := structured.(map[string]any)
	if !ok {
		t.Fatalf("structured MCP output = %#v", structured)
	}
	board, ok := root["board"].(map[string]any)
	if !ok {
		t.Fatalf("structured board output = %#v", root)
	}
	version, ok := board["version"].(float64)
	if !ok {
		t.Fatalf("structured board version = %#v", board["version"])
	}
	apps, ok := board["apps"].([]any)
	if !ok {
		t.Fatalf("structured board apps = %#v", board["apps"])
	}
	return dashboard.Snapshot{Version: uint64(version), Apps: make([]dashboard.AppView, len(apps))}
}
