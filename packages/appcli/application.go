package appcli

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"os/signal"

	"github.com/MikeO7/kinosail/packages/owneraccess"
	"github.com/MikeO7/kinosail/packages/publicgateway"
)

// Application adapts one Player-derived process to the shared command lifecycle.
type Application[Configuration Settings] struct {
	Configuration func([]string, io.Writer) (bool, error)
	Load          func() (Configuration, error)
	MCP           func(context.Context, Configuration, string) error
	Command       Command
	AuthURL       func(Configuration) string
	Run           func(Configuration) int
}

// Execute runs Player's command and server-mode dispatch without terminating the process.
func Execute[Configuration Settings](args []string, input io.Reader, output io.Writer, getenv func(string) string, signals []os.Signal, application Application[Configuration]) int { //nolint:cyclop // Keep command precedence explicit; the repository complexity gate still applies.
	// Dispatch before loading application settings or opening private state.
	if len(args) > 0 && args[0] == "public-gateway" {
		return executePublicGateway(args, getenv, signals)
	}
	ConfigureMCPRuntime(args)
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stderr, nil)))
	if !application.valid() || getenv == nil {
		slog.Error("application is not configured")
		return 1
	}
	if handled, err := application.Configuration(args, output); handled {
		return commandResult("configuration failed", err)
	}
	configured, configErr := application.Load()
	if len(args) > 0 && args[0] == "owner-access-disable" {
		if len(args) != 1 || configErr != nil {
			return 1
		}
		return commandResult("private management recovery failed", owneraccess.Recover(configured.String("paths.data")))
	}
	if len(args) >= 1 && len(args) <= 2 && args[0] == "mcp-stdio" {
		return executeMCP(args, signals, application, configured, configErr)
	}
	handled, err := application.Command(args, input, output, configured.String("paths.data"), configured.String("backup.key"), configured.String("listen"), configured.Bool("tls.enabled"), application.AuthURL(configured))
	if handled {
		return commandResult("command failed", err)
	}
	if configErr != nil {
		slog.Error("configuration failed", "error", configErr)
		return 1
	}
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: LogLevel(configured.String("logging.level"))})))
	if !SecureProxyConfiguration(configured.String("remote.proxy_token"), getenv("KINOSAIL_HOSTNAME")) {
		slog.Error("remote access requires a 32-character proxy capability")
		return 1
	}
	return application.Run(configured)
}

func commandResult(message string, err error) int {
	if err == nil {
		return 0
	}
	slog.Error(message, "error", err)
	return 1
}

// SecureProxyConfiguration validates the proxy capability at the process boundary.
func SecureProxyConfiguration(token, remote string) bool {
	return remote == "" && token == "" || len(token) >= 32
}

func executeMCP[Configuration Settings](args []string, signals []os.Signal, application Application[Configuration], configured Configuration, configErr error) int {
	if configErr != nil {
		slog.Error("configuration failed", "error", configErr)
		return 1
	}
	ctx, stop := signal.NotifyContext(context.Background(), signals...)
	defer stop()
	profileID := ""
	if len(args) == 2 {
		profileID = args[1]
	}
	if err := application.MCP(ctx, configured, profileID); err != nil && !errors.Is(err, context.Canceled) {
		slog.Error("command failed", "error", err)
		return 1
	}
	return 0
}

func (a Application[Configuration]) valid() bool {
	return a.Configuration != nil && a.Load != nil && a.MCP != nil && a.Command != nil && a.AuthURL != nil && a.Run != nil
}

func executePublicGateway(args []string, getenv func(string) string, signals []os.Signal) int {
	if len(args) != 1 || getenv == nil {
		return 1
	}
	ctx, stop := signal.NotifyContext(context.Background(), signals...)
	defer stop()
	return commandResult("public gateway stopped", publicgateway.Run(ctx, getenv("KINOSAIL_PUBLIC_HOSTNAME")))
}
