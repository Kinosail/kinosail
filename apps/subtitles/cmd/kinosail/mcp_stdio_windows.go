//go:build windows

package main

import (
	"context"
	"os"

	"github.com/MikeO7/kinosail-subtitles/internal/server"
)

func serveMCPStdioCommand(ctx context.Context, config server.Config, profileID string) error {
	return server.ServeMCPStdio(ctx, config, profileID, os.Stdin, os.Stdout)
}
