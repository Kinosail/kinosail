package main

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/MikeO7/kinosail-player/internal/backup"
	"github.com/MikeO7/kinosail/packages/commandtest"
)

func TestCLIContract(t *testing.T) {
	commandtest.RunCLI(t, commandtest.CLI{
		Command: command, WriteEncrypted: backup.WriteEncrypted,
		ConfigureRuntime: configureMCPStdioRuntime,
		LogLevel:         configuredLogLevel, SecureProxy: secureProxyConfiguration,
		Listen: "127.0.0.1:38127",
	}, loadConfiguration, newConfiguredHTTPServerWithAccess, serveConfiguredMCP)
}

func TestCommandLineHelperProcess(t *testing.T) {
	if os.Getenv("KINOSAIL_TLS_TEST") == "1" {
		_, _ = fmt.Fprintf(os.Stderr, "TLS test helper entered after package initialization at %s\n", time.Now().UTC().Format(time.RFC3339Nano))
	}
	commandtest.HelperProcess(t, main)
}
