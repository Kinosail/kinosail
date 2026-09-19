package appcli

import (
	"time"

	"github.com/MikeO7/kinosail/packages/auditjournal"
	"github.com/MikeO7/kinosail/packages/federation"
	"github.com/MikeO7/kinosail/packages/mcpgateway"
	"github.com/MikeO7/kinosail/packages/metadata"
	"github.com/MikeO7/kinosail/packages/scim"
)

// ServerSettings exposes the typed values needed by both HTTP applications.
type ServerSettings interface {
	String(string) string
	Int(string) int
	Duration(string) time.Duration
}

// ServerValues contains the shared part of each application's server wiring.
type ServerValues struct {
	MediaDir, DataDir, CacheDir string
	BackupDir, BackupKey        string
	BackupInterval              time.Duration
	BackupRetention             int
	FFmpeg, FFprobe, FPCalc     string
	AuthURL, ProxyToken         string
	ScanInterval                time.Duration
	DLNAURL                     string
	Metadata                    metadata.Config
	OIDC                        federation.OIDCConfig
	SAML                        federation.SAMLConfig
	SCIM                        scim.Config
	MCP                         mcpgateway.OAuthConfig
	Notifications               auditjournal.NotificationConfig
	SupporterActivationURL      string
	SupportURL                  string
}

// BuildServerValues translates the shared configuration surface once.
func BuildServerValues(config ServerSettings, authURL string) ServerValues {
	dataDir := config.String("paths.data")
	values := ServerValues{
		MediaDir:               config.String("paths.media"),
		DataDir:                dataDir,
		CacheDir:               config.String("paths.cache"),
		BackupDir:              config.String("backup.directory"),
		BackupKey:              config.String("backup.key"),
		BackupInterval:         config.Duration("backup.interval"),
		BackupRetention:        config.Int("backup.retention"),
		FFmpeg:                 config.String("binaries.ffmpeg"),
		FFprobe:                config.String("binaries.ffprobe"),
		FPCalc:                 config.String("binaries.fpcalc"),
		AuthURL:                authURL,
		ProxyToken:             config.String("remote.proxy_token"),
		ScanInterval:           config.Duration("scanning.interval"),
		DLNAURL:                config.String("dlna.url"),
		Metadata:               metadata.Config{URL: config.String("integrations.tmdb.url"), ImageURL: config.String("integrations.tmdb.image_url"), Token: config.String("integrations.tmdb.token")},
		OIDC:                   federation.OIDCConfig{Issuer: config.String("integrations.oidc.issuer"), ClientID: config.String("integrations.oidc.client_id"), ClientSecret: config.String("integrations.oidc.client_secret"), RedirectURL: config.String("integrations.oidc.redirect_url"), IdentityClaim: config.String("integrations.oidc.identity_claim")},
		SAML:                   federation.SAMLConfig{MetadataURL: config.String("integrations.saml.metadata_url"), MetadataXML: config.String("integrations.saml.metadata_xml"), IdentityAttribute: config.String("integrations.saml.identity_attribute"), RootURL: authURL, DataDir: dataDir},
		SCIM:                   scim.Config{Token: config.String("integrations.scim.token"), TokenExpiresAt: parseTime(config.String("integrations.scim.token_expires_at"))},
		MCP:                    mcpgateway.OAuthConfig{ResourceURL: config.String("integrations.mcp.resource_url"), AuthorizationServer: config.String("integrations.mcp.authorization_server"), IntrospectionURL: config.String("integrations.mcp.introspection_url"), ClientID: config.String("integrations.mcp.client_id"), ClientSecret: config.String("integrations.mcp.client_secret")},
		Notifications:          auditjournal.NotificationConfig{URL: config.String("integrations.webhook.url"), Token: config.String("integrations.webhook.token")},
		SupporterActivationURL: config.String("supporter.activation_url"),
		SupportURL:             config.String("supporter.url"),
	}
	return values
}

func parseTime(value string) time.Time {
	parsed, _ := time.Parse(time.RFC3339, value)
	return parsed
}
