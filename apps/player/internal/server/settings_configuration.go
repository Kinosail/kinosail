package server

import (
	"errors"
	"net/http"
	"strings"

	"github.com/MikeO7/kinosail-player/internal/configuration"
	settingsops "github.com/MikeO7/kinosail/packages/settings"
)

const configurationHTML = `<!doctype html><html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width"><script src="/static/theme.js?v=electric-1"></script><link rel="stylesheet" href="/static/app.css?v=skeleton-3"><title>Configuration · Kinosail Player</title></head><body class="settings-page"><main class="settings-shell"><a class="back" href="/settings">{{icon "back"}} Settings</a><header class="settings-intro"><span class="eyebrow">Owner controls</span><h1>Advanced configuration</h1><p>Change settings normally managed by Docker, environment variables, or YAML. Changes saved here apply the next time Kinosail Server starts.</p></header>{{range .}}<section id="{{.Key}}"><h2>{{.Label}}</h2>{{if eq .Source "environment"}}<p>Set by your deployment environment.</p>{{else if eq .Source "yaml"}}<p>Set in your YAML configuration file.</p>{{else if eq .Source "gui"}}<p>Using a value saved here.</p>{{else}}<p>Using the Kinosail default.</p>{{end}}{{if .Secret}}<p>{{if .Configured}}A secret is configured. Its value is hidden.{{else}}No secret is configured.{{end}}</p>{{end}}{{if or (eq .Source "environment") (eq .Source "yaml")}}<p>To change this setting, update the external value and restart Kinosail Server.</p>{{else}}<form action="/settings/configuration" method="post"><input type="hidden" name="key" value="{{.Key}}"><label>{{if .Secret}}New secret{{else}}Value{{end}}<input aria-label="{{.Label}}" name="value" {{if .Secret}}type="password" autocomplete="off" placeholder="New secret"{{else}}value="{{.Value}}"{{end}} required></label><button>Save change</button><p>Applies after a restart.</p></form>{{if eq .Source "gui"}}<form action="/settings/configuration/reset" method="post"><button class="quiet" name="key" value="{{.Key}}">Use default</button></form>{{end}}{{end}}<details><summary>Technical details</summary><p>Configuration key: <code>{{.Key}}</code><br>Environment variable: <code>{{.Env}}</code></p></details></section>{{end}}</main></body></html>`

var configurationView = newLocalizedTemplate("configuration", ignoreNonPasswordSecretAutofill(settingControlHTML+strings.NewReplacer("Set by your deployment environment.", `Configured via Docker: <code>{{.Env}}</code>.`, "</header>", "</header>"+tmdbConfigurationHTML+oidcConfigurationHTML+samlConfigurationHTML+scimConfigurationHTML, "{{range .}}", "{{range .Fields}}").Replace(configurationHTML)))

type configurationField struct {
	configuration.PublicValue
	Label string
}

type configurationPage struct {
	Fields []configurationField
	TMDB   tmdbConfigurationView
	OIDC   oidcConfigurationView
	SAML   samlConfigurationView
	SCIM   scimConfigurationView
}

type settingControl struct {
	Managed       bool
	DockerVars    []string
	YAMLKeys      []string
	ConnectionURL string
}

func configurationControl(configured configuration.Snapshot, keys ...string) settingControl {
	control := settingControl{}
	for _, key := range keys {
		field := configured.Public(key)
		switch field.Source {
		case configuration.Environment:
			control.Managed = true
			control.DockerVars = append(control.DockerVars, field.Env)
		case configuration.YAML:
			control.Managed = true
			control.YAMLKeys = append(control.YAMLKeys, field.Key)
		case configuration.Default, configuration.GUI:
		}
	}
	return control
}

var configurationLabels = map[string]string{ //nolint:gosec // These are display labels for secret settings, not credentials.
	"auth.url": "Sign-in address", "backup.directory": "Backup folder", "backup.interval": "Backup frequency", "backup.key": "Backup encryption key", "backup.retention": "Backups to keep",
	"binaries.ffmpeg": "FFmpeg command", "binaries.ffprobe": "FFprobe command", "binaries.fpcalc": "Chromaprint command", "dlna.url": "DLNA address",
	"integrations.mcp.authorization_server": "MCP authorization server", "integrations.mcp.client_id": "MCP client ID", "integrations.mcp.client_secret": "MCP client secret", "integrations.mcp.introspection_url": "MCP token check address", "integrations.mcp.resource_url": "MCP resource address",
	"integrations.oidc.client_id": "Single sign-on client ID", "integrations.oidc.client_secret": "Single sign-on client secret", "integrations.oidc.issuer": "Single sign-on issuer", "integrations.oidc.redirect_url": "Single sign-on return address",
	"integrations.saml.metadata_url": "SAML provider metadata URL", "integrations.saml.metadata_xml": "SAML provider metadata XML",
	"integrations.scim.token":            "SCIM provisioning token",
	"integrations.scim.token_expires_at": "SCIM provisioning token expiration",
	"integrations.tmdb.image_url":        "TMDB image address", "integrations.tmdb.token": "TMDB access token", "integrations.tmdb.url": "TMDB address", "integrations.webhook.token": "Webhook token", "integrations.webhook.url": "Webhook address",
	"listen": "Server listening address", "logging.audit_retention": "Audit log retention", "logging.level": "Log detail level", "logging.playback_retention": "Playback history retention",
	"paths.cache": "Cache folder", "paths.media": "Media folder", "remote.proxy_token": "Trusted proxy token",
	"remote.gateway": "Restricted public gateway", "remote.mode": "Secure remote access mode", "remote.duckdns_domain": "DuckDNS subdomain", "remote.duckdns_token": "DuckDNS token", "remote.listen": "Public HTTPS listening address",
	"scanning.interval": "Library scan frequency", "supporter.activation_url": "Supporter activation address", "supporter.url": "Support Kinosail", "tls.enabled": "HTTPS enabled", "tls.hosts": "Accepted HTTPS names",
	"tls.duckdns": "Trusted HTTPS",
}

func (store *settingsStore) deploymentFields() []configurationField {
	store.mu.RLock()
	defer store.mu.RUnlock()
	result := make([]configurationField, 0)
	for _, field := range store.config.Fields() {
		if field.Restart && field.Key != "paths.data" && field.Key != "tls.duckdns" && field.Key != tmdbTokenKey && !strings.HasPrefix(field.Key, oidcConfigurationKey+".") && !strings.HasPrefix(field.Key, samlConfigurationGroupKey+".") && !strings.HasPrefix(field.Key, scimConfigurationKey+".") {
			view := configurationField{PublicValue: field, Label: configurationLabels[field.Key]}
			result = append(result, view)
		}
	}
	return result
}

func (store *settingsStore) changeConfiguration(key, value string, reset bool) error {
	if key == tmdbTokenKey {
		return errors.New("TMDB access is changed through its dedicated settings operation")
	}
	if key == "integrations.scim.token" || key == "integrations.scim.token_expires_at" {
		if !reset {
			return errors.New("SCIM token and expiration must be changed together")
		}
		return store.changeSCIMConfiguration("", "", true)
	}
	configured := settingsops.ConfigurationStore[configuration.Source]{
		File: store.file, Lock: &store.mu, Managed: store.config.Managed, Source: store.config.Source,
		Field: func(key string) settingsops.ConfigurationField {
			field := store.config.Public(key)
			return settingsops.ConfigurationField{Known: field.Key != "", Restart: field.Restart}
		},
		Set: configuration.Set, Delete: configuration.Delete, Update: store.config.UpdateGUI,
	}
	return configured.Change(key, value, reset)
}

func showConfiguration(settings *settingsStore) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := configurationView.Execute(writer, request, configurationPage{Fields: settings.deploymentFields(), TMDB: settings.tmdbConfiguration(), OIDC: settings.oidcConfiguration(), SAML: settings.samlConfiguration(request), SCIM: settings.scimConfiguration(request)}); err != nil {
			localizedError(writer, request, err.Error(), http.StatusInternalServerError)
		}
	}
}

func saveConfiguration(settings *settingsStore, reset bool) http.HandlerFunc { //nolint:cyclop,gocognit // Key-specific forms route to one validated application operation.
	return func(writer http.ResponseWriter, request *http.Request) {
		if !reset && request.FormValue("key") == tmdbConfigurationKey {
			saveTMDBConfiguration(writer, request, settings, "/settings/configuration#integrations.tmdb")
			return
		}
		if !reset && request.FormValue("key") == oidcConfigurationKey {
			saveOIDCConfiguration(writer, request, settings)
			return
		}
		if !reset && request.FormValue("key") == samlConfigurationGroupKey {
			saveSAMLConfiguration(writer, request, settings)
			return
		}
		if !reset && request.FormValue("key") == scimConfigurationKey {
			saveSCIMConfiguration(writer, request, settings)
			return
		}
		key, value, err := settingsops.ParseConfigurationForm(writer, request, reset)
		if err != nil {
			localizedError(writer, request, "invalid configuration form", http.StatusBadRequest)
			return
		}
		switch key {
		case tmdbConfigurationKey:
			err = settings.changeTMDBConfiguration(request.Context(), "", true)
		case oidcConfigurationKey:
			err = settings.changeOIDCConfiguration("", "", "", "", "", true)
		case samlConfigurationGroupKey:
			err = settings.changeSAMLConfiguration("", "", "", true)
		case scimConfigurationKey:
			err = settings.changeSCIMConfiguration("", "", true)
		default:
			err = settings.changeConfiguration(key, value, reset)
		}
		if err != nil {
			localizedError(writer, request, err.Error(), http.StatusConflict)
			return
		}
		http.Redirect(writer, request, "/settings/configuration", http.StatusSeeOther)
	}
}
