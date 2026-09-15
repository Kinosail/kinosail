//go:build !windows

package mcpgateway

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestRunStdioCommandRejectsInvalidDependencies(t *testing.T) {
	t.Parallel()
	validPath := filepath.Join(t.TempDir(), "socat")
	validServe := func() error { return nil }
	validOpen := func() (*os.File, error) { return nil, errors.New("stop") }
	validExec := func(string, []string, []string) error { return nil }
	for name, test := range map[string]struct {
		path  string
		serve func() error
		open  func() (*os.File, error)
		exec  StdioCommandExec
	}{
		"empty path":    {"", validServe, validOpen, validExec},
		"relative path": {"socat", validServe, validOpen, validExec},
		"unclean path":  {validPath + "/..", validServe, validOpen, validExec},
		"long path":     {"/" + strings.Repeat("x", 4096), validServe, validOpen, validExec},
		"missing serve": {validPath, nil, validOpen, validExec},
		"missing open":  {validPath, validServe, nil, validExec},
		"missing exec":  {validPath, validServe, validOpen, nil},
	} {
		if err := RunStdioCommand(test.path, test.serve, test.open, test.exec); err == nil {
			t.Errorf("%s configuration was accepted", name)
		}
	}
}

func TestRunStdioCommandFallsBackWhenRelayIsUnavailable(t *testing.T) {
	t.Parallel()
	want := errors.New("served directly")
	called := false
	err := RunStdioCommand(filepath.Join(t.TempDir(), "missing"), func() error { called = true; return want }, func() (*os.File, error) { t.Fatal("relay opened"); return nil, nil }, func(string, []string, []string) error { t.Fatal("relay executed"); return nil })
	if !called || !errors.Is(err, want) {
		t.Fatalf("fallback = called %v, error %v", called, err)
	}
}

func TestRunStdioCommandValidatesAndExecutesRelay(t *testing.T) { //nolint:cyclop,funlen // Each relay resource boundary is verified together.
	t.Parallel()
	directory := t.TempDir()
	path := filepath.Join(directory, "socat")
	if err := os.WriteFile(path, []byte("relay"), 0o600); err != nil {
		t.Fatal(err)
	}
	wantOpen := errors.New("open failed")
	if err := RunStdioCommand(path, func() error { return nil }, func() (*os.File, error) { return nil, wantOpen }, func(string, []string, []string) error { return nil }); !errors.Is(err, wantOpen) {
		t.Fatalf("open error = %v", err)
	}
	if err := RunStdioCommand(path, func() error { return nil }, func() (*os.File, error) { return nil, nil }, func(string, []string, []string) error { return nil }); err == nil {
		t.Fatal("nil relay socket was accepted")
	}
	closed, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := closed.Close(); err != nil {
		t.Fatal(err)
	}
	if err := RunStdioCommand(path, func() error { return nil }, func() (*os.File, error) { return closed, nil }, func(string, []string, []string) error { return nil }); err == nil {
		t.Fatal("closed relay descriptor was accepted")
	}

	relay, peer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer peer.Close()
	wantExec := errors.New("exec stopped")
	err = RunStdioCommand(path, func() error { return nil }, func() (*os.File, error) { return relay, nil }, func(gotPath string, arguments, environment []string) error {
		if gotPath != path || len(arguments) != 3 || arguments[0] != "socat" || arguments[1] != "STDIO" || !strings.HasPrefix(arguments[2], "FD:") || !slices.Equal(environment, os.Environ()) {
			t.Fatalf("exec = %q, %#v, %#v", gotPath, arguments, environment)
		}
		return wantExec
	})
	if !errors.Is(err, wantExec) {
		t.Fatalf("exec error = %v", err)
	}
	if _, err := relay.Stat(); err == nil {
		t.Fatal("relay socket remained open")
	}
}
