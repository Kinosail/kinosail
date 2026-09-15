//go:build !windows

package main

import (
	"testing"

	"github.com/MikeO7/kinosail-player/internal/server"
	"github.com/MikeO7/kinosail/packages/commandtest"
)

func TestConfiguredMCPBindsBothStdioPaths(t *testing.T) {
	commandtest.StdioBindings(t, server.Config{DataDir: t.TempDir()}, server.ServeMCPStdio, server.OpenMCPStdioRelay)
}
