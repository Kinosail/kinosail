//go:build !windows

package mcpgateway

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestBoundStdioCommandPreservesApplicationArguments(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "relay")
	want := errors.New("application stopped")
	served, opened := false, false
	serve := func(ctx context.Context, config int, profile string, input io.Reader, output io.Writer) error {
		assertBoundApplication(t, ctx, config, profile)
		if input != os.Stdin || output != os.Stdout {
			t.Fatal("direct command changed standard streams")
		}
		served = true
		return want
	}
	open := func(ctx context.Context, config int, profile string) (*os.File, error) {
		assertBoundApplication(t, ctx, config, profile)
		opened = true
		return nil, want
	}
	execute := func(string, []string, []string) error {
		t.Fatal("failed application command executed relay")
		return nil
	}
	run := BindStdioCommand(path, serve, open, execute)
	if err := run(t.Context(), 7, "owner"); !errors.Is(err, want) || !served || opened {
		t.Fatalf("direct path: error=%v served=%v opened=%v", err, served, opened)
	}
	if err := os.WriteFile(path, []byte("relay"), 0o600); err != nil {
		t.Fatal(err)
	}
	served = false
	if err := run(t.Context(), 7, "owner"); !errors.Is(err, want) || served || !opened {
		t.Fatalf("relay path: error=%v served=%v opened=%v", err, served, opened)
	}
}

func assertBoundApplication(t *testing.T, ctx context.Context, config int, profile string) {
	t.Helper()
	if ctx != t.Context() || config != 7 || profile != "owner" {
		t.Fatal("command changed application arguments")
	}
}

func TestBoundStdioCommandRejectsMissingOperations(t *testing.T) {
	t.Parallel()
	serve := func(context.Context, int, string, io.Reader, io.Writer) error {
		t.Fatal("invalid binding served")
		return nil
	}
	open := func(context.Context, int, string) (*os.File, error) {
		t.Fatal("invalid binding opened relay")
		return nil, nil
	}
	for _, run := range []func(context.Context, int, string) error{
		BindStdioCommand[int]("/relay", nil, open, nil),
		BindStdioCommand[int]("/relay", serve, nil, nil),
		BindStdioCommand[int]("/relay", serve, open, nil),
	} {
		if err := run(t.Context(), 7, "owner"); err == nil {
			t.Fatal("invalid application binding was accepted")
		}
	}
}
