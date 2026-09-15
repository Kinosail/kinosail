package main

import (
	"testing"

	"github.com/MikeO7/kinosail-subtitles/internal/backup"
	"github.com/MikeO7/kinosail/packages/commandtest"
)

func TestCLIContract(t *testing.T) {
	commandtest.RunCLI(t, commandtest.CLI{
		Command: command, WriteEncrypted: backup.WriteEncrypted,
		ConfigureRuntime: configureMCPStdioRuntime,
		LogLevel:         configuredLogLevel, SecureProxy: secureProxyConfiguration,
		Listen: "127.0.0.1:38128",
	}, loadConfiguration, newConfiguredHTTPServerWithAccess, serveConfiguredMCP)
}

func TestCommandLineHelperProcess(t *testing.T) {
	commandtest.HelperProcess(t, main)
}
