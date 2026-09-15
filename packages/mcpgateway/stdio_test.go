//go:build !windows

package mcpgateway

import (
	"bufio"
	"bytes"
	"context"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func stdioFixture(t *testing.T, principals *testPrincipals) *Gateway {
	t.Helper()
	api := &testAPI{respond: func(writer http.ResponseWriter, _ *http.Request) {
		writeJSON(writer, map[string]string{"name": "Owner"}, http.StatusOK)
	}}
	return newGateway(testGatewayConfig(OAuthConfig{}, principals, api, testRoutes{
		ReadAccess: {"GET /api/v1/me": true}, WriteAccess: {}, ManageAccess: {},
	}))
}

func TestStdioTransportUsesOwnerAndSharedTools(t *testing.T) { //nolint:cyclop,funlen // One lifecycle covers host, selector, bridge, tools, and shutdown.
	owner := Principal{ID: "owner", Name: "Owner", Owner: true}
	principals := &testPrincipals{values: map[string]Principal{"owner": owner}}
	dataDir := t.TempDir()
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	if err := StartStdioHost(ctx, dataDir, stdioFixture(t, principals), principals); err != nil {
		t.Fatal(err)
	}
	path, err := StdioSocketPath(dataDir)
	if err != nil {
		t.Fatal(err)
	}
	if info, err := os.Stat(path); err != nil || info.Mode()&os.ModeSocket == 0 || info.Mode().Perm() != 0o600 {
		t.Fatalf("socket = %#v %v", info, err)
	}
	serverReader, clientWriter := io.Pipe()
	clientReader, serverWriter := io.Pipe()
	done := make(chan error, 1)
	go func() { done <- ServeStdio(ctx, dataDir, "", serverReader, serverWriter) }()
	client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "1"}, nil)
	session, err := client.Connect(ctx, &mcp.IOTransport{Reader: clientReader, Writer: clientWriter}, nil)
	if err != nil {
		t.Fatal(err)
	}
	tools, err := session.ListTools(ctx, nil)
	if err != nil || len(tools.Tools) < 5 {
		t.Fatalf("tools = %#v %v", tools, err)
	}
	result, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "read_api", Arguments: map[string]any{"path": "/api/v1/me"}})
	if err != nil || result.IsError || !bytes.Contains([]byte(result.Content[0].(*mcp.TextContent).Text), []byte("Owner")) {
		t.Fatalf("read_api = %#v %v", result, err)
	}
	cancel()
	_ = session.Close()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("STDIO bridge did not stop")
	}
}

func TestStdioRelayAndSelectionFailClosed(t *testing.T) { //nolint:cyclop,funlen // Selector and relay checks share one host fixture.
	first := Principal{ID: "first", Name: "First", Owner: true}
	second := Principal{ID: "second", Name: "Second", Owner: true}
	principals := &testPrincipals{values: map[string]Principal{"first": first, "second": second, "viewer": {ID: "viewer"}}}
	if _, err := mcpStdioOwner(principals, ""); err == nil {
		t.Fatal("ambiguous Owner was accepted")
	}
	selected, err := mcpStdioOwner(principals, second.ID)
	if err != nil || selected != second {
		t.Fatalf("selected = %#v %v", selected, err)
	}
	for _, id := range []string{"viewer", "missing", "bad\nselector", strings.Repeat("x", 129)} {
		if _, err := mcpStdioOwner(principals, id); err == nil {
			t.Fatalf("selector %q was accepted", id)
		}
	}
	principals.err = io.ErrUnexpectedEOF
	if _, err := mcpStdioOwner(principals, second.ID); err == nil || !strings.Contains(err.Error(), "state is unavailable") {
		t.Fatalf("profile state error = %v", err)
	}
	principals.err = nil
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	dataDir := t.TempDir()
	if err := StartStdioHost(ctx, dataDir, stdioFixture(t, principals), principals); err != nil {
		t.Fatal(err)
	}
	if err := StartStdioHost(ctx, dataDir, stdioFixture(t, principals), principals); err == nil || !strings.Contains(err.Error(), "already in use") {
		t.Fatalf("duplicate host = %v", err)
	}
	relay, err := OpenStdioRelay(ctx, dataDir, second.ID)
	if err != nil {
		t.Fatal(err)
	}
	connection, err := net.FileConn(relay)
	_ = relay.Close()
	if err != nil {
		t.Fatal(err)
	}
	defer connection.Close()
	client := mcp.NewClient(&mcp.Implementation{Name: "relay", Version: "1"}, nil)
	session, err := client.Connect(ctx, &mcp.IOTransport{Reader: connection, Writer: connection}, nil)
	if err != nil {
		t.Fatal(err)
	}
	_ = session.Close()
}

func TestStdioRejectsInvalidInputsWithoutSideEffects(t *testing.T) { //nolint:cyclop // One table-free test proves each invalid transport input is side-effect free.
	dataDir := t.TempDir()
	if _, err := StdioSocketPath(""); err == nil {
		t.Fatal("empty data directory was accepted")
	}
	if err := ServeStdio(nil, dataDir, "", strings.NewReader(""), io.Discard); err == nil { //nolint:staticcheck // This call verifies the documented nil-context rejection.
		t.Fatal("nil context was accepted")
	}
	if err := ServeStdio(t.Context(), dataDir, "", nil, io.Discard); err == nil {
		t.Fatal("nil input was accepted")
	}
	if err := ServeStdio(t.Context(), dataDir, "", strings.NewReader(""), nil); err == nil {
		t.Fatal("nil output was accepted")
	}
	if err := ServeStdio(t.Context(), dataDir, "bad selector", strings.NewReader(""), io.Discard); err == nil {
		t.Fatal("invalid selector was accepted")
	}
	if err := ServeStdio(t.Context(), dataDir, "", strings.NewReader(""), io.Discard); err == nil || !strings.Contains(err.Error(), "running Kinosail Server") {
		t.Fatalf("missing host = %v", err)
	}
	entries, err := os.ReadDir(dataDir)
	if err != nil || len(entries) != 0 {
		t.Fatalf("rejected inputs changed data: %#v %v", entries, err)
	}
	if err := StartStdioHost(t.Context(), dataDir, nil, nil); err == nil {
		t.Fatal("missing host adapters were accepted")
	}
}

func TestStdioCapacityIsBounded(t *testing.T) {
	owner := Principal{ID: "owner", Owner: true}
	principals := &testPrincipals{values: map[string]Principal{"owner": owner}}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	dataDir := t.TempDir()
	if err := StartStdioHost(ctx, dataDir, stdioFixture(t, principals), principals); err != nil {
		t.Fatal(err)
	}
	path, _ := StdioSocketPath(dataDir)
	connections := make([]net.Conn, 0, mcpStdioConnectionLimit)
	defer func() {
		for _, connection := range connections {
			_ = connection.Close()
		}
	}()
	for range mcpStdioConnectionLimit {
		connection, err := (&net.Dialer{}).DialContext(ctx, "unix", path)
		if err != nil {
			t.Fatal(err)
		}
		connections = append(connections, connection)
		_ = connection.SetDeadline(time.Now().Add(time.Second))
		_, _ = io.WriteString(connection, "\n")
		if response, _ := bufio.NewReader(connection).ReadString('\n'); response != "OK\n" {
			t.Fatalf("handshake = %q", response)
		}
		_ = connection.SetDeadline(time.Time{})
	}
	if err := ServeStdio(ctx, dataDir, "", strings.NewReader(""), io.Discard); err == nil || err.Error() != "MCP STDIO connection capacity reached" {
		t.Fatalf("capacity = %v", err)
	}
}
