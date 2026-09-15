//go:build !windows

package commandtest

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/MikeO7/kinosail/packages/mcpgateway"
)

// StdioBindings verifies both real application operations without replacing a process.
func StdioBindings[Config any](t *testing.T, config Config, serve func(context.Context, Config, string, io.Reader, io.Writer) error, open func(context.Context, Config, string) (*os.File, error)) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	path := filepath.Join(t.TempDir(), "relay")
	execute := func(string, []string, []string) error {
		t.Fatal("canceled command executed relay")
		return nil
	}
	run := mcpgateway.BindStdioCommand(path, serve, open, execute)
	if err := run(ctx, config, ""); err == nil {
		t.Fatal("canceled direct server path succeeded")
	}
	if err := os.WriteFile(path, []byte("relay"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := run(ctx, config, ""); err == nil {
		t.Fatal("canceled relay path succeeded")
	}
}
