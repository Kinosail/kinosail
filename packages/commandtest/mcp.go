package commandtest

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func MCPOwner(t *testing.T, newServer func(context.Context) *http.Server) {
	dataDir := t.TempDir()
	t.Setenv("KINOSAIL_DATA_DIR", dataDir)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	httpServer := newServer(ctx)
	app := httpServer.Handler
	setup := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/setup", strings.NewReader("name=Owner&password=owner-password"))
	setup.Host = httpServer.Addr
	setup.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response := httptest.NewRecorder()
	app.ServeHTTP(response, setup)
	if response.Code != http.StatusSeeOther {
		t.Fatalf("setup = %d %q", response.Code, response.Body.String())
	}

	//nolint:gosec // G204: the executable is the current Go test binary, not external input.
	command := exec.CommandContext(t.Context(), os.Args[0], "-test.run=TestCommandLineHelperProcess", "--", "mcp-stdio")
	command.Env = append(os.Environ(), "KINOSAIL_CLI_HELPER=1", "KINOSAIL_DATA_DIR="+dataDir)
	client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "1"}, nil)
	session, err := client.Connect(t.Context(), &mcp.CommandTransport{Command: command}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	tools, err := session.ListTools(t.Context(), nil)
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
			t.Errorf("mcp-stdio omits %q", name)
		}
	}
}

func MCPRuntime(t *testing.T, configureMCPStdioRuntime func([]string)) {
	procs := runtime.GOMAXPROCS(0)
	t.Cleanup(func() { runtime.GOMAXPROCS(procs) })
	configureMCPStdioRuntime([]string{"mcp-stdio"})
	if runtime.GOMAXPROCS(0) != 1 {
		t.Fatalf("MCP STDIO GOMAXPROCS = %d", runtime.GOMAXPROCS(0))
	}
	runtime.GOMAXPROCS(procs)
	configureMCPStdioRuntime([]string{"version"})
	if runtime.GOMAXPROCS(0) != procs {
		t.Fatalf("Server GOMAXPROCS = %d, want %d", runtime.GOMAXPROCS(0), procs)
	}
}

func MCPCanceled(t *testing.T, serve func(context.Context) error) {
	t.Setenv("KINOSAIL_DATA_DIR", t.TempDir())
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if err := serve(ctx); err == nil {
		t.Fatal("canceled MCP STDIO command succeeded without a running server")
	}
}
