package server

import (
	"errors"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/MikeO7/kinosail-subtitles/internal/configuration"
)

const configurationHTML = `<!doctype html><html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width"><script src="/static/theme.js?v=electric-1"></script><link rel="stylesheet" href="/static/app.css?v=electric-1"><title>Configuration · Kinosail Subtitles</title></head><body class="settings-page"><main class="settings-shell"><a class="back" href="/settings">{{icon "back"}} Settings</a><header class="settings-intro"><span class="eyebrow">Owner controls</span><h1>Advanced configuration</h1><p>Change settings normally managed by Docker, environment variables, or YAML. Changes saved here apply the next time Kinosail Server starts.</p></header>{{range .}}<section id="{{.Key}}"><h2>{{.Label}}</h2>{{if eq .Source "environment"}}<p>Set by your deployment environment.</p>{{else if eq .Source "yaml"}}<p>Set in your YAML configuration file.</p>{{else if eq .Source "gui"}}<p>Using a value saved here.</p>{{else}}<p>Using the Kinosail default.</p>{{end}}{{if .Secret}}<p>{{if .Configured}}A secret is configured. Its value is hidden.{{else}}No secret is configured.{{end}}</p>{{end}}{{if or (eq .Source "environment") (eq .Source "yaml")}}<p>To change this setting, update the external value and restart Kinosail Server.</p>{{else}}<form action="/settings/configuration" method="post"><input type="hidden" name="key" value="{{.Key}}"><label>{{if .Secret}}New secret{{else}}Value{{end}}<input aria-label="{{.Label}}" name="value" {{if .Secret}}type="password" autocomplete="off" placeholder="New secret"{{else}}value="{{.Value}}"{{end}} required></label><button>Save change</button><p>Applies after a restart.</p></form>{{if eq .Source "gui"}}<form action="/settings/configuration/reset" method="post"><button class="quiet" name="key" value="{{.Key}}">Use default</button></form>{{end}}{{end}}<details><summary>Technical details</summary><p>Configuration key: <code>{{.Key}}</code><br>Environment variable: <code>{{.Env}}</code></p></details></section>{{end}}</main></body></html>`

var configurationView = newLocalizedTemplate("configuration", ignoreNonPasswordSecretAutofill(strings.NewReplacer("Set by your deployment environment.", `Configured via Docker: <code>{{.Env}}</code>.`, "</header>", "</header>"+openSubtitlesConfigurationHTML+oidcConfigurationHTML+samlConfigurationHTML+scimConfigurationHTML, "{{range .}}", "{{range .Fields}}", "app.css?v=46", "app.css?v=64").Replace(configurationHTML)))

type configurationField struct {
	configuration.PublicValue
	Label string
}
type configurationPage struct {
	Fields        []configurationField
	OpenSubtitles openSubtitlesConfigurationView
	OIDC          oidcConfigurationView
	SAML          samlConfigurationView
	SCIM          scimConfigurationView
}
type settingControl struct {
	Managed              bool
	DockerVars, YAMLKeys []string
	ConnectionURL        string
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
	"integrations.scim.token":             "SCIM provisioning token",
	"integrations.scim.token_expires_at":  "SCIM provisioning token expiration",
	"integrations.opensubtitles.api_key":  "OpenSubtitles API key",
	"integrations.opensubtitles.password": "OpenSubtitles password",
	"integrations.opensubtitles.url":      "OpenSubtitles address",
	"integrations.opensubtitles.username": "OpenSubtitles username",
	"integrations.subdl.api_key":          "SubDL API key",
	"integrations.subdl.url":              "SubDL address",
	"integrations.tmdb.image_url":         "TMDB image address", "integrations.tmdb.token": "TMDB access token", "integrations.tmdb.url": "TMDB address", "integrations.webhook.token": "Webhook token", "integrations.webhook.url": "Webhook address",
	"listen": "Server listening address", "logging.audit_retention": "Audit log retention", "logging.level": "Log detail level", "logging.playback_retention": "Playback history retention",
	"paths.cache": "Cache folder", "paths.media": "Media folder", "remote.proxy_token": "Trusted proxy token",
	"remote.mode": "Secure remote access mode", "remote.duckdns_domain": "DuckDNS subdomain", "remote.duckdns_token": "DuckDNS token", "remote.listen": "Public HTTPS listening address",
	"scanning.interval": "Library scan frequency", "supporter.activation_url": "Supporter activation address", "supporter.url": "Support Kinosail", "tls.enabled": "HTTPS enabled", "tls.hosts": "Accepted HTTPS names",
	"tls.duckdns": "Trusted HTTPS",
}

func (store *settingsStore) deploymentFields() []configurationField {
	store.mu.RLock()
	defer store.mu.RUnlock()
	result := make([]configurationField, 0)
	for _, field := range store.config.Fields() {
		if field.Restart && field.Key != "paths.data" && field.Key != "tls.duckdns" && !strings.HasPrefix(field.Key, openSubtitlesConfigurationKey+".") && !strings.HasPrefix(field.Key, oidcConfigurationKey+".") && !strings.HasPrefix(field.Key, samlConfigurationGroupKey+".") && !strings.HasPrefix(field.Key, scimConfigurationKey+".") && !strings.HasPrefix(field.Key, subSourceConfigurationKey+".") {
			view := configurationField{PublicValue: field, Label: configurationLabels[field.Key]}
			result = append(result, view)
		}
	}
	return result
}

func (store *settingsStore) changeConfiguration(key, value string, reset bool) error {
	if key == "integrations.scim.token" || key == "integrations.scim.token_expires_at" {
		if !reset {
			return errors.New("SCIM token and expiration must be changed together")
		}
		return store.changeSCIMConfiguration("", "", true)
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	return store.changeConfigurationLocked(key, value, reset)
}

func (store *settingsStore) changeConfigurationLocked(key, value string, reset bool) error {
	if store.file == "" {
		return errors.New("configuration storage is unavailable")
	}
	if store.config.Managed(key) {
		return errors.New(key + " is managed by " + string(store.config.Source(key)))
	}
	if key == "paths.data" {
		return errors.New("paths.data must be set through YAML or KINOSAIL_DATA_DIR")
	}
	if key == "tls.duckdns" {
		return errors.New("setting is changed through its dedicated settings operation")
	}
	field := store.config.Public(key)
	if field.Key == "" || !field.Restart {
		return errors.New("setting is changed through its dedicated settings operation")
	}
	if reset {
		if err := configuration.Delete(filepath.Dir(store.file), key); err != nil {
			return err
		}
		store.config.UpdateGUI(key, "", true)
		return nil
	}
	if err := configuration.Set(filepath.Dir(store.file), key, value); err != nil {
		return err
	}
	store.config.UpdateGUI(key, value, false)
	return nil
}

func showConfiguration(settings *settingsStore) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := configurationView.Execute(writer, request, configurationPage{Fields: settings.deploymentFields(), OpenSubtitles: settings.openSubtitlesConfiguration(), OIDC: settings.oidcConfiguration(), SAML: settings.samlConfiguration(request), SCIM: settings.scimConfiguration(request)}); err != nil {
			localizedError(writer, request, err.Error(), http.StatusInternalServerError)
		}
	}
}

func saveConfiguration(settings *settingsStore, reset bool) http.HandlerFunc { //nolint:cyclop,gocognit // Key-specific forms route to one validated application operation.
	return func(writer http.ResponseWriter, request *http.Request) {
		if !reset && request.FormValue("key") == oidcConfigurationKey {
			saveOIDCConfiguration(writer, request, settings)
			return
		}
		if !reset && request.FormValue("key") == openSubtitlesConfigurationKey {
			saveOpenSubtitlesConfiguration(writer, request, settings)
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
		key, value, ok := configurationForm(request, reset)
		if !ok {
			localizedError(writer, request, "invalid configuration form", http.StatusBadRequest)
			return
		}
		var err error
		switch key {
		case oidcConfigurationKey:
			err = settings.changeOIDCConfiguration("", "", "", "", "", true)
		case samlConfigurationGroupKey:
			err = settings.changeSAMLConfiguration("", "", "", true)
		case scimConfigurationKey:
			err = settings.changeSCIMConfiguration("", "", true)
		case openSubtitlesConfigurationKey:
			err = settings.changeOpenSubtitlesConfiguration("", "", "", true)
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

func configurationForm(request *http.Request, reset bool) (string, string, bool) {
	if !formEncoded(request) || request.URL.RawQuery != "" || request.ParseForm() != nil {
		return "", "", false
	}
	key, keyOK := oneValue(request.PostForm, "key", 128)
	if !keyOK {
		return "", "", false
	}
	if reset {
		return key, "", onlyFormKeys(request.PostForm, "key")
	}
	value, valueOK := oneValue(request.PostForm, "value", 16<<10)
	return key, value, valueOK && onlyFormKeys(request.PostForm, "key", "value")
}
