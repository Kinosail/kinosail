// Package appcli defines Player's process-level command contract for media apps.
package appcli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"time"

	"github.com/MikeO7/kinosail/packages/backup"
	"github.com/MikeO7/kinosail/packages/identitycore"
	"github.com/MikeO7/kinosail/packages/remoteaccess"
	"github.com/MikeO7/kinosail/packages/servertransport"
	"github.com/MikeO7/kinosail/packages/trustedhttps"
	"github.com/MikeO7/kinosail/packages/updatecontrol"
)

// Command dispatches one process command.
type Command func([]string, io.Reader, io.Writer, string, string, string, bool, string) (bool, error)

// Bind returns Player's command dispatcher with app-specific backup and update adapters.
func Bind(backupCommand backup.Command, updateCommand updatecontrol.Command, version string) Command {
	if backupCommand == nil || updateCommand == nil {
		return func([]string, io.Reader, io.Writer, string, string, string, bool, string) (bool, error) {
			return true, errors.New("command adapters are not configured")
		}
	}
	return func(args []string, input io.Reader, output io.Writer, dataDir, backupKey, listen string, tlsEnabled bool, authURL string) (bool, error) {
		return dispatch(args, input, output, dataDir, backupKey, listen, tlsEnabled, authURL, backupCommand, updateCommand, version)
	}
}

func dispatch(args []string, input io.Reader, output io.Writer, dataDir, backupKey, listen string, tlsEnabled bool, authURL string, backupCommand backup.Command, updateCommand updatecontrol.Command, version string) (bool, error) {
	if len(args) == 0 {
		return false, nil
	}
	if unsafeArgs(args) {
		return true, errors.New("invalid command arguments")
	}
	if handled, err := backupCommand(args, input, output, dataDir, backupKey); handled {
		return true, err
	}
	if len(args) != 1 {
		return true, usageError()
	}
	return builtInCommand(args[0], input, output, dataDir, listen, tlsEnabled, authURL, updateCommand, version)
}

func builtInCommand(name string, input io.Reader, output io.Writer, dataDir, listen string, tlsEnabled bool, authURL string, updateCommand updatecontrol.Command, version string) (bool, error) {
	switch name {
	case "healthcheck":
		return true, servertransport.CheckHealth(listen, tlsEnabled, authURL)
	case "tls-certificate":
		return true, servertransport.WriteTrustAnchor(output, dataDir)
	case updatecontrol.CommandArtifact, updatecontrol.CommandPlan, updatecontrol.CommandReport:
		return true, updateCommand(name, input, output, dataDir, runtime.GOOS, runtime.GOARCH)
	case "version":
		_, err := fmt.Fprintln(output, version)
		return true, err
	default:
		return true, fmt.Errorf("unknown command %q", name)
	}
}

func unsafeArgs(args []string) bool {
	if len(args) > 3 {
		return true
	}
	for _, argument := range args {
		if len(argument) > 4096 {
			return true
		}
	}
	return false
}

func usageError() error {
	return errors.New("usage: kinosail [backup|config|healthcheck|mcp-stdio|restore|tls-certificate|update-artifact|update-plan|update-report|version]")
}

// ConfigureMCPRuntime reserves the process for a bounded STDIO server.
func ConfigureMCPRuntime(args []string) {
	if len(args) >= 1 && len(args) <= 2 && args[0] == "mcp-stdio" {
		runtime.GOMAXPROCS(1)
	}
}

// LogLevel resolves Player's validated logging choice.
func LogLevel(value string) slog.Level {
	return map[string]slog.Level{"debug": slog.LevelDebug, "warn": slog.LevelWarn, "error": slog.LevelError}[value]
}

// Settings is the process configuration used by Player-derived applications.
type Settings interface {
	Bool(string) bool
	String(string) string
	Strings(string) []string
}

// ServerBuilder creates an application's HTTP server around the shared access managers.
type ServerBuilder func(context.Context, *remoteaccess.Manager, *trustedhttps.Manager) *http.Server

// Run owns Player's server lifecycle for Player-derived applications.
func Run(configured Settings, signals []os.Signal, build ServerBuilder, remoteHandler func(http.Handler) http.Handler) int {
	return run(configured, signals, build, remoteHandler, RemoteAccess, TrustedHTTPS, servertransport.Serve)
}

type (
	remoteFactory  func(Settings) (*remoteaccess.Manager, error)
	trustedFactory func(Settings) (*trustedhttps.Manager, error)
	serverRunner   func(*http.Server, servertransport.TLSConfig) error
)

func run(configured Settings, signals []os.Signal, build ServerBuilder, remoteHandler func(http.Handler) http.Handler, createRemote remoteFactory, createTrusted trustedFactory, serve serverRunner) int { //nolint:cyclop // Optional managers, remote serving, and shutdown form one process lifecycle.
	if configured == nil || build == nil || remoteHandler == nil || createRemote == nil || createTrusted == nil || serve == nil {
		slog.Error("server lifecycle is not configured")
		return 1
	}
	ctx, stop := signal.NotifyContext(context.Background(), signals...)
	defer stop()
	internet, trusted := accessManagers(configured, createRemote, createTrusted)
	httpServer := build(ctx, internet, trusted)
	if httpServer == nil {
		slog.Error("server lifecycle returned no HTTP server")
		return 1
	}
	httpServer.Handler = identitycore.PrivateNetwork(httpServer.Handler)
	startAccessManagers(ctx, configured.String("remote.mode"), httpServer, internet, trusted, remoteHandler)
	go shutdownOnCancel(ctx, httpServer)

	slog.Info("Kinosail ready", "listen", httpServer.Addr, "tls", configured.Bool("tls.enabled"))
	if err := serve(httpServer, servertransport.TLSConfig{Enabled: configured.Bool("tls.enabled"), DataDir: configured.String("paths.data"), Hosts: configured.Strings("tls.hosts"), Certificates: trusted}); err != nil && !errors.Is(err, http.ErrServerClosed) {
		slog.Error("server failed", "error", err)
		return 1
	}
	return 0
}

func accessManagers(configured Settings, createRemote remoteFactory, createTrusted trustedFactory) (*remoteaccess.Manager, *trustedhttps.Manager) {
	internet, err := createRemote(configured)
	if err != nil {
		slog.Error("secure remote access failed", "error", err)
		internet = nil
	}
	trusted, err := createTrusted(configured)
	if err != nil {
		slog.Error("trusted HTTPS failed", "error", err)
		trusted = nil
	}
	return internet, trusted
}

func startAccessManagers(ctx context.Context, remoteMode string, server *http.Server, internet *remoteaccess.Manager, trusted *trustedhttps.Manager, remoteHandler func(http.Handler) http.Handler) {
	if trusted != nil {
		go trusted.Run(ctx)
	}
	if remoteMode != "off" && internet != nil {
		go func() {
			if err := internet.ServeContinuously(ctx, remoteHandler(server.Handler)); err != nil && !errors.Is(err, context.Canceled) {
				slog.Error("secure remote access stopped", "error", err)
			}
		}()
	}
}

func shutdownOnCancel(ctx context.Context, server *http.Server) {
	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		slog.Error("shutdown failed", "error", err)
	}
}

// RemoteAccess creates Player's validated remote-access manager.
func RemoteAccess(configured Settings) (*remoteaccess.Manager, error) {
	mode := configured.String("remote.mode")
	return remoteaccess.New(remoteaccess.Config{Enabled: mode != "off", PublicHTTPS: mode == "https", Gateway: configured.Bool("remote.gateway"), Domain: configured.String("remote.duckdns_domain"), Token: configured.String("remote.duckdns_token"), Listen: configured.String("remote.listen"), DataDir: configured.String("paths.data")})
}

// TrustedHTTPS creates Player's validated trusted-HTTPS manager.
func TrustedHTTPS(configured Settings) (*trustedhttps.Manager, error) {
	trusted, err := trustedhttps.Parse(configured.String("tls.duckdns"))
	if err != nil || trusted == (trustedhttps.Config{}) {
		return nil, err
	}
	return trustedhttps.New(trusted, configured.String("paths.data"))
}
