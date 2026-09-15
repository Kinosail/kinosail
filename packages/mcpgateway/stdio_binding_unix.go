//go:build !windows

package mcpgateway

import (
	"context"
	"errors"
	"io"
	"os"
)

// BindStdioCommand connects application operations to the shared process handoff.
func BindStdioCommand[Config any](path string, serve func(context.Context, Config, string, io.Reader, io.Writer) error, open func(context.Context, Config, string) (*os.File, error), execute StdioCommandExec) func(context.Context, Config, string) error {
	return func(ctx context.Context, config Config, profileID string) error {
		if serve == nil || open == nil {
			return errors.New("invalid MCP STDIO application configuration")
		}
		return RunStdioCommand(path,
			func() error { return serve(ctx, config, profileID, os.Stdin, os.Stdout) },
			func() (*os.File, error) { return open(ctx, config, profileID) }, execute)
	}
}
