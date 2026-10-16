package main

import (
	"context"
	"io"
	"net"
	"net/http"
	"os"
	"sync"

	"github.com/MikeO7/kinosail-player/internal/backup"
	"github.com/MikeO7/kinosail-player/internal/configuration"
	"github.com/MikeO7/kinosail-player/internal/database"
	"github.com/MikeO7/kinosail-player/internal/server"
	"github.com/MikeO7/kinosail/packages/appcli"
	"github.com/MikeO7/kinosail/packages/remoteaccess"
	"github.com/MikeO7/kinosail/packages/servertransport"
	"github.com/MikeO7/kinosail/packages/trustedhttps"
	"github.com/MikeO7/kinosail/packages/updatecontrol"
)

var version = "dev"

func main() {
	backup.ApplicationVersion = version
	server.ApplicationVersion = version
	exit := appcli.Execute(os.Args[1:], os.Stdin, os.Stdout, os.Getenv, terminationSignals(), application())
	if exit != 0 {
		os.Exit(exit)
	}
}

func application() appcli.Application[configuration.Snapshot] {
	return appcli.Application[configuration.Snapshot]{Configuration: executeConfigurationCommand, Load: loadConfiguration, MCP: serveConfiguredMCP, Command: command, AuthURL: configuredAuthURL, Run: run}
}

var (
	warmLocalizedViews = server.WarmLocalizedViews
	updateCommand      = updatecontrol.BindCommand(updatecontrol.CommandConfig{
		Policy:       updatecontrol.PlayerPolicy(database.SchemaVersion, configuration.SchemaVersion),
		OpenDatabase: openUpdateDatabase,
		Version:      func() string { return version },
	})
	command                  = appcli.Bind(backup.Command, updateCommand, version)
	configureMCPStdioRuntime = appcli.ConfigureMCPRuntime
	configuredLogLevel       = appcli.LogLevel
	secureProxyConfiguration = appcli.SecureProxyConfiguration
)

func openUpdateDatabase(dataDir string) (updatecontrol.CommandDatabase, error) {
	return database.Open(dataDir, false)
}

func executeConfigurationCommand(args []string, output io.Writer) (bool, error) {
	return configurationCommand(args, output, loadConfigurationPath)
}

func serveConfiguredMCP(ctx context.Context, configured configuration.Snapshot, profileID string) error {
	return serveMCPStdioCommand(ctx, configuredMCPServerConfig(configured), profileID)
}

func run(configured configuration.Snapshot) int {
	return appcli.Run(configured, terminationSignals(), func(ctx context.Context, drain *sync.WaitGroup, internet *remoteaccess.Manager, trusted *trustedhttps.Manager) *http.Server {
		return newConfiguredHTTPServerWithAccess(ctx, drain, configured, internet, trusted)
	}, server.Remote)
}

func newConfiguredHTTPServerWithAccess(ctx context.Context, drain *sync.WaitGroup, configured configuration.Snapshot, internet *remoteaccess.Manager, trusted *trustedhttps.Manager) *http.Server {
	config := configuredServerConfig(ctx, configured, internet, trusted)
	config.Drain = drain
	handler := server.New(config) //nolint:contextcheck // Durable state commits intentionally finish independently of request cancellation.
	httpServer := servertransport.NewServer(configured.String("listen"), handler)
	warmAfterListening(ctx, httpServer)
	return httpServer
}

func warmAfterListening(ctx context.Context, httpServer *http.Server) {
	var once sync.Once
	httpServer.BaseContext = func(net.Listener) context.Context {
		once.Do(func() { go func() { _ = warmLocalizedViews(ctx) }() })
		return context.Background()
	}
}

func configuredServerConfig(ctx context.Context, configured configuration.Snapshot, internet *remoteaccess.Manager, trusted *trustedhttps.Manager) server.Config {
	values := appcli.BuildServerValues(configured, configuredAuthURL(configured))
	config := server.Config{
		Lifecycle: ctx, MediaDir: values.MediaDir, DataDir: values.DataDir, CacheDir: values.CacheDir,
		BackupDir: values.BackupDir, BackupKey: values.BackupKey, BackupInterval: values.BackupInterval, BackupRetention: values.BackupRetention,
		FFmpeg: values.FFmpeg, FFprobe: values.FFprobe, ProbeHardware: true, FPCalc: values.FPCalc, RequireAuth: true,
		AuthURL: values.AuthURL, ProxyToken: values.ProxyToken, ScanInterval: values.ScanInterval, DLNAURL: values.DLNAURL,
		Configuration: configured, Supporter: server.SupporterConfig{ActivationURL: values.SupporterActivationURL, SupportURL: values.SupportURL},
		Metadata: values.Metadata, OIDC: values.OIDC, SAML: values.SAML, SCIM: values.SCIM, MCP: values.MCP, Notifications: values.Notifications,
	}
	config.InternetAccess, config.TrustedHTTPS = internet, trusted
	return config
}

func configuredMCPServerConfig(configured configuration.Snapshot) server.Config {
	return server.Config{DataDir: configured.String("paths.data")}
}

func loadConfiguration() (configuration.Snapshot, error) {
	return loadConfigurationPath("")
}

func loadConfigurationPath(path string) (configuration.Snapshot, error) {
	dataDir := env("KINOSAIL_DATA_DIR", "/config")
	if path == "" {
		path = os.Getenv("KINOSAIL_CONFIG_FILE")
	}
	if path == "" {
		candidate := dataDir + "/kinosail.yaml"
		if _, err := os.Stat(candidate); err == nil {
			path = candidate
		}
	}
	return configuration.Load(dataDir, path, os.LookupEnv)
}

func env(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
