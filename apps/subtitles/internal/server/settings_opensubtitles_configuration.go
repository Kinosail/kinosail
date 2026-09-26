package server

import (
	"errors"
	"net/http"
	"path/filepath"

	"github.com/MikeO7/kinosail-subtitles/internal/configuration"
)

const (
	openSubtitlesConfigurationKey  = "integrations.opensubtitles"
	openSubtitlesConfigurationHTML = `{{with .OpenSubtitles}}<section class="wide" id="integrations.opensubtitles"><h2>OpenSubtitles</h2>{{if .Control.Managed}}<p>OpenSubtitles is configured outside Kinosail. Update the API key, username, and password together, then restart Kinosail Server.</p>{{range .Control.DockerVars}}<p>Configured via Docker: <code>{{.}}</code>.</p>{{end}}{{range .Control.YAMLKeys}}<p>Configured via YAML: <code>{{.}}</code>.</p>{{end}}{{else}}<p>{{if .Configured}}OpenSubtitles credentials are configured.{{else}}OpenSubtitles is not configured.{{end}}</p><form action="/settings/configuration" method="post"><input type="hidden" name="key" value="integrations.opensubtitles"><div class="oidc-fields"><label>API key<input aria-label="OpenSubtitles API key" name="apiKey" type="password" maxlength="4096" autocomplete="off" placeholder="{{if .Configured}}Leave blank to keep the configured API key{{else}}API key{{end}}" {{if not .Configured}}required{{end}}></label><label>Username<input aria-label="OpenSubtitles username" name="username" maxlength="4096" autocomplete="username" spellcheck="false" placeholder="{{if .Configured}}Leave blank to keep the configured username{{else}}Username{{end}}" {{if not .Configured}}required{{end}}></label><label>Password<input aria-label="OpenSubtitles password" name="password" type="password" maxlength="4096" autocomplete="new-password" placeholder="{{if .Configured}}Leave blank to keep the configured password{{else}}Password{{end}}" {{if not .Configured}}required{{end}}></label></div><p>Save all three values together. They will be hidden after you leave this page.</p><button>Save OpenSubtitles</button><p>Changes take effect immediately.</p></form>{{if .Configured}}<form action="/settings/configuration/reset" method="post"><button class="quiet" name="key" value="integrations.opensubtitles">Disable OpenSubtitles</button></form>{{end}}{{end}}<details><summary>Technical details</summary><p>Configuration keys: <code>integrations.opensubtitles.api_key</code>, <code>integrations.opensubtitles.username</code>, <code>integrations.opensubtitles.password</code>.</p></details></section>{{end}}`
)

var openSubtitlesConfigurationKeys = []string{"integrations.opensubtitles.api_key", "integrations.opensubtitles.username", "integrations.opensubtitles.password"}

type openSubtitlesConfigurationView struct {
	Configured bool
	Control    settingControl
}

func (store *settingsStore) openSubtitlesConfiguration() openSubtitlesConfigurationView {
	store.mu.RLock()
	defer store.mu.RUnlock()
	return openSubtitlesConfigurationView{store.config.Public("integrations.opensubtitles.api_key").Configured, configurationControl(store.config, openSubtitlesConfigurationKeys...)}
}

func (store *settingsStore) changeOpenSubtitlesConfiguration(apiKey, username, password string, reset bool) error { //nolint:cyclop,gocognit // Provider configuration is validated and persisted atomically.
	store.mu.Lock()
	defer store.mu.Unlock()
	if store.file == "" {
		return errors.New("configuration storage is unavailable")
	}
	for _, key := range openSubtitlesConfigurationKeys {
		if store.config.Managed(key) {
			return errors.New(key + " is managed by " + string(store.config.Source(key)))
		}
	}
	dataDir := filepath.Dir(store.file)
	if reset {
		if err := configuration.DeleteOpenSubtitles(dataDir); err != nil {
			return err
		}
		for _, key := range openSubtitlesConfigurationKeys {
			store.config.UpdateGUI(key, "", true)
		}
		store.refreshSubtitleProviderLocked(openSubtitlesConfigurationKey)
		return nil
	}
	values := map[string]*string{"integrations.opensubtitles.api_key": &apiKey, "integrations.opensubtitles.username": &username, "integrations.opensubtitles.password": &password}
	for key, value := range values {
		if *value == "" {
			*value = store.config.String(key)
		}
	}
	if err := configuration.SetOpenSubtitles(dataDir, apiKey, username, password); err != nil {
		return err
	}
	for key, value := range map[string]string{"integrations.opensubtitles.api_key": apiKey, "integrations.opensubtitles.username": username, "integrations.opensubtitles.password": password} {
		store.config.UpdateGUI(key, value, false)
	}
	store.refreshSubtitleProviderLocked(openSubtitlesConfigurationKey)
	return nil
}

func saveOpenSubtitlesConfiguration(writer http.ResponseWriter, request *http.Request, settings *settingsStore) {
	apiKey, apiKeyOK := firstBounded(request.PostForm["apiKey"], 4096, false)
	username, usernameOK := firstBounded(request.PostForm["username"], 4096, false)
	password, passwordOK := firstBounded(request.PostForm["password"], 4096, false)
	if !formEncoded(request) || request.URL.RawQuery != "" || !apiKeyOK || !usernameOK || !passwordOK || !onlyFormKeys(request.PostForm, "key", "apiKey", "username", "password") {
		localizedError(writer, request, "invalid OpenSubtitles form", http.StatusBadRequest)
		return
	}
	if err := settings.changeOpenSubtitlesConfiguration(apiKey, username, password, false); err != nil {
		localizedError(writer, request, err.Error(), http.StatusConflict)
		return
	}
	http.Redirect(writer, request, "/settings/configuration#integrations.opensubtitles", http.StatusSeeOther)
}

func apiChangeOpenSubtitlesConfiguration(writer http.ResponseWriter, request *http.Request, settings *settingsStore, reset bool) {
	var input struct {
		APIKey   string `json:"apiKey"`
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if !reset && !readJSON(writer, request, &input) {
		return
	}
	if err := settings.changeOpenSubtitlesConfiguration(input.APIKey, input.Username, input.Password, reset); err != nil {
		apiError(writer, err, http.StatusConflict)
		return
	}
	writeJSON(writer, map[string]any{"status": "saved", "restartRequired": false}, http.StatusAccepted)
}
