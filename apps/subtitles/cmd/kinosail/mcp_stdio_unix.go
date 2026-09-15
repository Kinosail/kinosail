//go:build !windows

package main

import (
	"syscall"

	"github.com/MikeO7/kinosail-subtitles/internal/server"
	"github.com/MikeO7/kinosail/packages/mcpgateway"
)

var serveMCPStdioCommand = mcpgateway.BindStdioCommand("/usr/bin/socat", server.ServeMCPStdio, server.OpenMCPStdioRelay, syscall.Exec)
