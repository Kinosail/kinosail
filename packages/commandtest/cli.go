package commandtest

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"testing"

	"github.com/MikeO7/kinosail/packages/appcli"
)

// CLI binds real application operations to the shared command-line scenarios.
type CLI struct {
	Command          appcli.Command
	WriteEncrypted   func(io.Writer, string, string) error
	ConfigureRuntime func([]string)
	ServeMCP         func(context.Context) error
	LogLevel         func(string) slog.Level
	SecureProxy      func(string, string) bool
	HTTPServer       func(context.Context) *http.Server
	Listen           string
}

// RunCLI binds the typed configuration operations before running shared scenarios.
func RunCLI[Snapshot, Internet, Trusted any](t *testing.T, cli CLI, load func() (Snapshot, error), build func(context.Context, Snapshot, *Internet, *Trusted) *http.Server, serve func(context.Context, Snapshot, string) error) {
	cli.HTTPServer = HTTPFactory(load, build)
	cli.ServeMCP = func(ctx context.Context) error {
		configured, err := load()
		if err != nil {
			return err
		}
		return serve(ctx, configured, "")
	}
	cli.Run(t)
}

func (cli CLI) Run(t *testing.T) {
	t.Run("TestBackupCommandsRoundTrip", func(t *testing.T) { BackupRoundTrip(t, cli.Command) })
	t.Run("TestRestoreCommandAcceptsScheduledEncryptedBackup", func(t *testing.T) { EncryptedRestore(t, cli.Command, cli.WriteEncrypted) })
	t.Run("TestRecoveryBackupIncludesPrivateStateAndVerifiesBeforeRestore", func(t *testing.T) { RecoveryBackup(t, cli.Command) })
	t.Run("TestVersionCommand", func(t *testing.T) { Version(t, cli.Command) })
	t.Run("TestMCPStdioCommandConnectsAsSoleOwner", func(t *testing.T) { MCPOwner(t, cli.HTTPServer) })
	t.Run("TestMCPStdioRuntimeReservesServerCapacity", func(t *testing.T) { MCPRuntime(t, cli.ConfigureRuntime) })
	t.Run("TestMCPStdioCommandStopsWhenCanceled", func(t *testing.T) { MCPCanceled(t, cli.ServeMCP) })
	t.Run("TestConfiguredLogLevels", func(t *testing.T) { LogLevels(t, cli.LogLevel) })
	t.Run("TestRemoteAccessRequiresStrongProxyCapability", func(t *testing.T) { ProxyCapability(t, cli.SecureProxy) })
	t.Run("TestHTTPServerBoundsSlowClientsWithoutTimingOutStreams", func(t *testing.T) { HTTPBounds(t, cli.HTTPServer, cli.Listen) })
	t.Run("TestHTTPTransportLogsRedactClientAddress", func(t *testing.T) { HTTPLogPrivacy(t, cli.HTTPServer) })
	t.Run("TestCommandLineEntrypointReportsVersionAndErrors", Entrypoint)
	t.Run("TestCommandLineServerReportsListenFailure", ListenFailure)
}
