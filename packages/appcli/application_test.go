package appcli

import (
	"context"
	"errors"
	"io"
	"runtime"
	"testing"
)

func TestExecuteDispatchesCommandsAndServerMode(t *testing.T) { //nolint:cyclop // One process-contract test covers precedence and exit codes.
	previous := runtime.GOMAXPROCS(0)
	defer runtime.GOMAXPROCS(previous)
	settings := testSettings{"paths.data": "/data", "backup.key": "key", "listen": ":1234", "logging.level": "info"}
	loaded := 0
	runExit := 7
	application := Application[testSettings]{
		Configuration: func([]string, io.Writer) (bool, error) { return false, nil },
		Load:          func() (testSettings, error) { loaded++; return settings, nil },
		MCP:           func(context.Context, testSettings, string) error { return nil },
		Command: func([]string, io.Reader, io.Writer, string, string, string, bool, string) (bool, error) {
			return false, nil
		},
		AuthURL: func(testSettings) string { return "https://media.example.com" },
		Run:     func(testSettings) int { return runExit },
	}
	if exit := Execute(nil, nil, io.Discard, func(string) string { return "" }, nil, application); exit != runExit || loaded != 1 {
		t.Fatalf("server mode = exit %d, loads %d", exit, loaded)
	}
	application.Command = func(_ []string, _ io.Reader, _ io.Writer, dataDir, backupKey, listen string, tls bool, authURL string) (bool, error) {
		if dataDir != "/data" || backupKey != "key" || listen != ":1234" || tls || authURL != "https://media.example.com" {
			t.Fatalf("command configuration = %q, %q, %q, %v, %q", dataDir, backupKey, listen, tls, authURL)
		}
		return true, nil
	}
	if exit := Execute([]string{"version"}, nil, io.Discard, func(string) string { return "" }, nil, application); exit != 0 {
		t.Fatalf("command exit = %d", exit)
	}
	profile := ""
	application.MCP = func(_ context.Context, _ testSettings, value string) error { profile = value; return context.Canceled }
	if exit := Execute([]string{"mcp-stdio", "owner"}, nil, io.Discard, func(string) string { return "" }, nil, application); exit != 0 || profile != "owner" {
		t.Fatalf("MCP = exit %d, profile %q", exit, profile)
	}
}

func TestExecuteFailurePaths(t *testing.T) {
	settings := testSettings{}
	want := errors.New("failed")
	base := func() Application[testSettings] {
		return Application[testSettings]{
			Configuration: func([]string, io.Writer) (bool, error) { return false, nil },
			Load:          func() (testSettings, error) { return settings, nil },
			MCP:           func(context.Context, testSettings, string) error { return nil },
			Command: func([]string, io.Reader, io.Writer, string, string, string, bool, string) (bool, error) {
				return false, nil
			},
			AuthURL: func(testSettings) string { return "" },
			Run:     func(testSettings) int { return 0 },
		}
	}
	application := base()
	application.Configuration = func([]string, io.Writer) (bool, error) { return true, want }
	if exit := Execute(nil, nil, io.Discard, func(string) string { return "" }, nil, application); exit != 1 {
		t.Fatalf("configuration command failure exit = %d", exit)
	}
	application = base()
	application.Load = func() (testSettings, error) { return settings, want }
	if exit := Execute(nil, nil, io.Discard, func(string) string { return "" }, nil, application); exit != 1 {
		t.Fatalf("configuration load failure exit = %d", exit)
	}
	if exit := Execute([]string{"mcp-stdio"}, nil, io.Discard, func(string) string { return "" }, nil, application); exit != 1 {
		t.Fatalf("MCP configuration failure exit = %d", exit)
	}
	application = base()
	application.MCP = func(context.Context, testSettings, string) error { return want }
	if exit := Execute([]string{"mcp-stdio"}, nil, io.Discard, func(string) string { return "" }, nil, application); exit != 1 {
		t.Fatalf("MCP failure exit = %d", exit)
	}
	application = base()
	application.Command = func([]string, io.Reader, io.Writer, string, string, string, bool, string) (bool, error) {
		return true, want
	}
	if exit := Execute([]string{"version"}, nil, io.Discard, func(string) string { return "" }, nil, application); exit != 1 {
		t.Fatalf("command failure exit = %d", exit)
	}
	application = base()
	settings["remote.proxy_token"] = "short"
	if exit := Execute(nil, nil, io.Discard, func(string) string { return "remote" }, nil, application); exit != 1 {
		t.Fatalf("proxy validation failure exit = %d", exit)
	}
	if exit := Execute(nil, nil, io.Discard, nil, nil, Application[testSettings]{}); exit != 1 {
		t.Fatalf("missing application exit = %d", exit)
	}
}

func TestSecureProxyConfiguration(t *testing.T) {
	t.Parallel()
	if SecureProxyConfiguration("short", "") || SecureProxyConfiguration("", "remote") || !SecureProxyConfiguration("", "") || !SecureProxyConfiguration("0123456789abcdef0123456789abcdef", "remote") {
		t.Fatal("secure proxy policy changed")
	}
}
