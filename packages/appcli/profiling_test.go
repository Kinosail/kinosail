//go:build !windows

package appcli

import (
	"context"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func profilingApplication(settings testSettings, run func(testSettings) int) Application[testSettings] {
	return Application[testSettings]{
		Configuration: func([]string, io.Writer) (bool, error) { return false, nil },
		Load:          func() (testSettings, error) { return settings, nil },
		MCP:           func(context.Context, testSettings, string) error { return nil },
		Command: func([]string, io.Reader, io.Writer, string, string, string, bool, string) (bool, error) {
			return false, nil
		},
		AuthURL: func(testSettings) string { return "" },
		Run:     run,
	}
}

func shortDataDir(t *testing.T) string {
	t.Helper()
	directory, err := os.MkdirTemp("", "ks")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(directory) })
	return directory
}

func socketGet(t *testing.T, socket, target string) (int, string) {
	t.Helper()
	client := http.Client{Transport: &http.Transport{DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, "unix", socket)
	}}}
	request, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "http://profiling"+target, nil)
	if err != nil {
		t.Fatal(err)
	}
	response, err := client.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	return response.StatusCode, string(body)
}

func TestProfilingIsServedOnlyOnAPrivateSocketWhenEnabled(t *testing.T) {
	dataDir := shortDataDir(t)
	socket := filepath.Join(dataDir, "profiling", "pprof.sock")
	settings := testSettings{"paths.data": dataDir, "logging.level": "info"}
	enabled := func(name string) string { return map[string]string{profilingVariable: "1"}[name] }
	exit := Execute(nil, nil, io.Discard, enabled, nil, profilingApplication(settings, func(testSettings) int {
		directory, err := os.Stat(filepath.Dir(socket))
		if err != nil || directory.Mode().Perm() != 0o700 {
			t.Fatalf("profiling directory = %v, %v", directory, err)
		}
		info, err := os.Stat(socket)
		if err != nil || info.Mode().Type() != os.ModeSocket || info.Mode().Perm() != 0o600 {
			t.Fatalf("profiling socket = %v, %v", info, err)
		}
		for target, want := range map[string]string{
			"/debug/pprof/":                  "goroutine",
			"/debug/pprof/heap?debug=1":      "heap profile",
			"/debug/pprof/cmdline":           "",
			"/debug/pprof/flight-recorder":   "go 1.",
			"/debug/pprof/goroutine?debug=1": "goroutine profile",
		} {
			if status, body := socketGet(t, socket, target); status != http.StatusOK || !strings.Contains(body, want) {
				t.Fatalf("%s = %d %.80q", target, status, body)
			}
		}
		return 0
	}))
	if exit != 0 {
		t.Fatalf("exit = %d", exit)
	}
	if _, err := os.Lstat(socket); !os.IsNotExist(err) {
		t.Fatalf("profiling socket remained after shutdown: %v", err)
	}
}

func TestProfilingStaysOffByDefaultAndRefusesNonSocketPaths(t *testing.T) {
	dataDir := shortDataDir(t)
	settings := testSettings{"paths.data": dataDir, "logging.level": "info"}
	Execute(nil, nil, io.Discard, func(string) string { return "" }, nil, profilingApplication(settings, func(testSettings) int { return 0 }))
	if _, err := os.Lstat(filepath.Join(dataDir, "profiling")); !os.IsNotExist(err) {
		t.Fatalf("profiling directory created while disabled: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(dataDir, "profiling"), 0o700); err != nil {
		t.Fatal(err)
	}
	occupied := filepath.Join(dataDir, "profiling", "pprof.sock")
	if err := os.WriteFile(occupied, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	enabled := func(name string) string { return map[string]string{profilingVariable: "1"}[name] }
	if exit := Execute(nil, nil, io.Discard, enabled, nil, profilingApplication(settings, func(testSettings) int { return 0 })); exit != 0 {
		t.Fatalf("server stopped because profiling was unavailable: %d", exit)
	}
	if contents, err := os.ReadFile(occupied); err != nil || string(contents) != "keep" {
		t.Fatalf("profiling replaced a regular file: %q %v", contents, err)
	}
}
