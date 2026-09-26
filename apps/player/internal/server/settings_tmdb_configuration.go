package server

import (
	"context"
	"errors"
	"net/http"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	"github.com/MikeO7/kinosail-player/internal/configuration"
	"github.com/MikeO7/kinosail/packages/httpguard"
)

const (
	tmdbConfigurationKey  = "integrations.tmdb"
	tmdbTokenKey          = "integrations.tmdb.token"
	tmdbDefaultURL        = "https://api.themoviedb.org/3"
	tmdbConfigurationHTML = `{{with .TMDB}}<section class="wide integration-setup" id="integrations.tmdb"><h2>Movie artwork and details</h2><p>Connect your own TMDB account so Kinosail can match movies and download posters, plots, genres, and cast details.</p><ol class="setup-steps"><li><a href="https://www.themoviedb.org/signup" target="_blank" rel="noopener noreferrer">Create a free TMDB account</a>, or <a href="https://www.themoviedb.org/login" target="_blank" rel="noopener noreferrer">sign in</a>.</li><li><a href="https://www.themoviedb.org/settings/api" target="_blank" rel="noopener noreferrer">Request API access</a>. Choose the use type that matches your Server.</li><li>Copy the <strong>API Read Access Token</strong>, then paste it below. Do not paste the shorter API Key.</li></ol><p><a href="https://developer.themoviedb.org/docs/getting-started" target="_blank" rel="noopener noreferrer">Open TMDB's key guide</a></p>{{template "configurationSource" .Control}}{{if .Control.Managed}}<p>TMDB access is configured outside Kinosail. Update the token there, then restart Kinosail Server.</p>{{range .Control.DockerVars}}<p>Configured via Docker: <code>{{.}}</code>.</p>{{end}}{{range .Control.YAMLKeys}}<p>Configured via YAML: <code>{{.}}</code>.</p>{{end}}{{else}}<p role="status">{{if .Configured}}TMDB access is configured.{{else}}TMDB access is not configured. Local artwork remains unchanged.{{end}}</p><form action="/settings/configuration" method="post"><input type="hidden" name="key" value="integrations.tmdb"><label>API Read Access Token<input aria-label="TMDB API Read Access Token" name="token" type="password" maxlength="4096" autocomplete="off" spellcheck="false" placeholder="{{if .Configured}}Leave blank to keep the configured token{{else}}Paste the API Read Access Token{{end}}" {{if not .Configured}}required{{end}}></label><button>Check and save TMDB access</button><p>Kinosail checks the token before saving it. Takes effect immediately. Kinosail starts filling missing artwork in the background.</p></form>{{if .Configured}}<form action="/settings/configuration/reset" method="post"><button class="quiet" name="key" value="integrations.tmdb">Remove TMDB access</button></form>{{end}}{{end}}<p>This product uses the TMDB API but is not endorsed or certified by TMDB.</p><details><summary>Technical details</summary><p>The token is hidden and stored in the protected secrets file. Configuration key: <code>integrations.tmdb.token</code>.</p></details></section>{{end}}`
)

func liveTMDBSetting(key string) bool {
	return key == tmdbTokenKey || key == "integrations.tmdb.url" || key == "integrations.tmdb.image_url"
}

type tmdbConfigurationView struct {
	Configured bool
	Control    settingControl
}

func (store *settingsStore) tmdbConfiguration() tmdbConfigurationView {
	store.mu.RLock()
	defer store.mu.RUnlock()
	return tmdbConfigurationView{store.config.Public(tmdbTokenKey).Configured, configurationControl(store.config, tmdbTokenKey)}
}

func validTMDBToken(token string) bool {
	return len(token) >= 32 && len(token) <= 4096 && strings.TrimSpace(token) == token && !strings.ContainsFunc(token, unicode.IsControl)
}

func checkTMDBToken(ctx context.Context, baseURL, token string) error {
	if baseURL != "" && strings.TrimRight(baseURL, "/") != tmdbDefaultURL {
		return errors.New("TMDB credentials require the official API address")
	}
	client := hardenedHTTPClient(15 * time.Second)
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, tmdbDefaultURL+"/configuration", nil)
	if err != nil {
		return errors.New("TMDB access could not be checked")
	}
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Accept", "application/json")
	response, err := client.Do(request)
	if err != nil {
		return errors.New("TMDB access could not be checked")
	}
	defer response.Body.Close()
	var result struct {
		Images struct {
			SecureBaseURL string `json:"secure_base_url"`
		} `json:"images"`
	}
	if response.StatusCode != http.StatusOK || decodeExternalJSON(response.Body, 1<<20, &result) != nil || result.Images.SecureBaseURL == "" {
		return errors.New("TMDB rejected the API Read Access Token")
	}
	return nil
}

func (store *settingsStore) changeTMDBConfiguration(ctx context.Context, token string, reset bool) error { //nolint:cyclop,gocognit // Validation and the guarded configuration transaction stay together.
	store.mu.RLock()
	file, baseURL, managed := store.file, store.config.String("integrations.tmdb.url"), store.config.Managed(tmdbTokenKey)
	store.mu.RUnlock()
	if file == "" {
		return errors.New("configuration storage is unavailable")
	}
	if managed {
		return errors.New(tmdbTokenKey + " is managed outside Kinosail")
	}
	if !reset {
		if !validTMDBToken(token) {
			return errors.New("enter a valid TMDB API Read Access Token")
		}
		if err := store.tmdbCheck(ctx, baseURL, token); err != nil {
			return err
		}
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	if store.config.Managed(tmdbTokenKey) || store.config.String("integrations.tmdb.url") != baseURL {
		return errors.New("TMDB configuration changed; try again")
	}
	if reset {
		if err := configuration.Delete(filepath.Dir(file), tmdbTokenKey); err != nil {
			return err
		}
		store.config.UpdateGUI(tmdbTokenKey, "", true)
		store.refreshMetadataProviderLocked()
		return nil
	}
	if err := configuration.Set(filepath.Dir(file), tmdbTokenKey, token); err != nil {
		return err
	}
	store.config.UpdateGUI(tmdbTokenKey, token, false)
	store.refreshMetadataProviderLocked()
	return nil
}

func (store *settingsStore) refreshMetadataProviderLocked() {
	if store.metadata != nil {
		store.metadata.configureTMDB(store.config.String(tmdbTokenKey), store.config.String("integrations.tmdb.url"), store.config.String("integrations.tmdb.image_url"))
	}
	if store.config.String(tmdbTokenKey) != "" && store.metadataChanged != nil {
		store.metadataChanged()
	}
}

func saveTMDBConfiguration(writer http.ResponseWriter, request *http.Request, settings *settingsStore, redirect string) {
	if !httpguard.FormEncoded(request) || request.URL.RawQuery != "" || request.ParseForm() != nil {
		localizedError(writer, request, "invalid TMDB configuration form", http.StatusBadRequest)
		return
	}
	token, tokenOK := httpguard.BoundedValue(request.PostForm["token"], 4096, false)
	if !tokenOK || !httpguard.OnlyFormKeys(request.PostForm, "key", "token") {
		localizedError(writer, request, "invalid TMDB configuration form", http.StatusBadRequest)
		return
	}
	if token == "" && settings.tmdbConfiguration().Configured {
		http.Redirect(writer, request, redirect, http.StatusSeeOther)
		return
	}
	if !validTMDBToken(token) {
		localizedError(writer, request, "enter a valid TMDB API Read Access Token", http.StatusBadRequest)
		return
	}
	if err := settings.changeTMDBConfiguration(request.Context(), token, false); err != nil {
		localizedError(writer, request, err.Error(), http.StatusConflict)
		return
	}
	http.Redirect(writer, request, redirect, http.StatusSeeOther)
}

func apiChangeTMDBConfiguration(writer http.ResponseWriter, request *http.Request, settings *settingsStore, reset bool) {
	var input struct {
		Token string `json:"token"`
	}
	if !reset && !readJSON(writer, request, &input) {
		return
	}
	if !reset && !validTMDBToken(input.Token) {
		apiError(writer, errors.New("enter a valid TMDB API Read Access Token"), http.StatusBadRequest)
		return
	}
	if err := settings.changeTMDBConfiguration(request.Context(), input.Token, reset); err != nil {
		apiError(writer, err, http.StatusConflict)
		return
	}
	writeJSON(writer, map[string]any{"status": "active", "restartRequired": false}, http.StatusAccepted)
}
