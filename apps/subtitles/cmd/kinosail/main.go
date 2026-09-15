package main

import (
	"context"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/MikeO7/kinosail-subtitles/internal/backup"
	"github.com/MikeO7/kinosail-subtitles/internal/configuration"
	"github.com/MikeO7/kinosail-subtitles/internal/database"
	"github.com/MikeO7/kinosail-subtitles/internal/server"
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
	exit := appcli.Execute(os.Args[1:], os.Stdin, os.Stdout, os.Getenv, terminationSignals(), appcli.Application[configuration.Snapshot]{Configuration: executeConfigurationCommand, Load: loadConfiguration, MCP: serveConfiguredMCP, Command: command, AuthURL: configuredAuthURL, Run: run})
	if exit != 0 {
		os.Exit(exit)
	}
}

var (
	updateCommand = updatecontrol.BindCommand(updatecontrol.CommandConfig{
		Policy:       updatecontrol.SubtitlesPolicy(database.SchemaVersion, configuration.SchemaVersion),
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

func run(configured configuration.Snapshot) int { //nolint:cyclop // Startup keeps optional access managers and graceful shutdown in one lifecycle.
	return appcli.Run(configured, terminationSignals(), func(ctx context.Context, internet *remoteaccess.Manager, trusted *trustedhttps.Manager) *http.Server {
		return newConfiguredHTTPServerWithAccess(ctx, configured, internet, trusted)
	}, server.Remote)
}

func newConfiguredHTTPServerWithAccess(ctx context.Context, configured configuration.Snapshot, internet *remoteaccess.Manager, trusted *trustedhttps.Manager) *http.Server {
	handler := server.New(configuredServerConfig(ctx, configured, internet, trusted)) //nolint:contextcheck // Durable state commits intentionally finish independently of request cancellation.
	return servertransport.NewServer(configured.String("listen"), handler)
}

func configuredServerConfig(ctx context.Context, configured configuration.Snapshot, internet *remoteaccess.Manager, trusted *trustedhttps.Manager) server.Config { //nolint:funlen // The single process transports share one application configuration.
	scimTokenExpiresAt, _ := time.Parse(time.RFC3339, configured.String("integrations.scim.token_expires_at"))
	config := server.Config{
		SubtitleApp:     true,
		Lifecycle:       ctx,
		MediaDir:        configured.String("paths.media"),
		DataDir:         configured.String("paths.data"),
		CacheDir:        configured.String("paths.cache"),
		BackupDir:       configured.String("backup.directory"),
		BackupKey:       configured.String("backup.key"),
		BackupInterval:  configured.Duration("backup.interval"),
		BackupRetention: configured.Int("backup.retention"),
		FFmpeg:          configured.String("binaries.ffmpeg"),
		FFprobe:         configured.String("binaries.ffprobe"),
		ProbeHardware:   true,
		FPCalc:          configured.String("binaries.fpcalc"),
		RequireAuth:     true,
		AuthURL:         configuredAuthURL(configured),
		ProxyToken:      configured.String("remote.proxy_token"),
		ScanInterval:    configured.Duration("scanning.interval"),
		DLNAURL:         configured.String("dlna.url"),
		Configuration:   configured,
		Supporter:       server.SupporterConfig{ActivationURL: configured.String("supporter.activation_url"), SupportURL: configured.String("supporter.url")},
		Subtitles:       configuredSubtitleProviders(configured),
		Metadata: server.MetadataConfig{
			URL:      configured.String("integrations.tmdb.url"),
			ImageURL: configured.String("integrations.tmdb.image_url"),
			Token:    configured.String("integrations.tmdb.token"),
		},
		OIDC: server.OIDCConfig{
			Issuer:        configured.String("integrations.oidc.issuer"),
			ClientID:      configured.String("integrations.oidc.client_id"),
			ClientSecret:  configured.String("integrations.oidc.client_secret"),
			RedirectURL:   configured.String("integrations.oidc.redirect_url"),
			IdentityClaim: configured.String("integrations.oidc.identity_claim"),
		},
		SAML: server.SAMLConfig{MetadataURL: configured.String("integrations.saml.metadata_url"), MetadataXML: configured.String("integrations.saml.metadata_xml"), IdentityAttribute: configured.String("integrations.saml.identity_attribute"), RootURL: configuredAuthURL(configured), DataDir: configured.String("paths.data")},
		SCIM: server.SCIMConfig{Token: configured.String("integrations.scim.token"), TokenExpiresAt: scimTokenExpiresAt},
		MCP: server.MCPConfig{
			ResourceURL:         configured.String("integrations.mcp.resource_url"),
			AuthorizationServer: configured.String("integrations.mcp.authorization_server"),
			IntrospectionURL:    configured.String("integrations.mcp.introspection_url"),
			ClientID:            configured.String("integrations.mcp.client_id"),
			ClientSecret:        configured.String("integrations.mcp.client_secret"),
		},
		Notifications: server.NotificationConfig{
			URL: configured.String("integrations.webhook.url"), Token: configured.String("integrations.webhook.token"),
		},
	}
	config.InternetAccess, config.TrustedHTTPS = internet, trusted
	if configured.String("remote.mode") == "wireguard" {
		config.WireGuardDir = configured.String("remote.wireguard_dir")
		config.WireGuardEndpoint = configured.String("remote.duckdns_domain") + ".duckdns.org:51820"
	}
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
