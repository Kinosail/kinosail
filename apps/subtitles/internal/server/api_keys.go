package server

import (
	"errors"
	"strings"

	"github.com/MikeO7/kinosail-subtitles/internal/database"
	"github.com/MikeO7/kinosail/packages/identitycore"
)

//nolint:gosec // G101: this is an HTML template for displaying a generated value once.
const apiKeyCreatedHTML = `<!doctype html><html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width"><title>API key · Kinosail Subtitles</title><script src="/static/theme.js"></script><link rel="stylesheet" href="/static/app.css"></head><body class="auth"><main class="grant-card"><span class="eyebrow">Save your API key</span><h1>{{.Name}}</h1><p>Copy this key now. You cannot view it again after leaving this page.</p><code class="grant-link">{{.Secret}}</code><p><a href="/settings">Return to settings</a></p></main></body></html>`

var apiKeySecretView = newLocalizedTemplate("api-key", apiKeyCreatedHTML)

type (
	apiKey     = identitycore.APIKey
	apiKeyView = identitycore.APIKeyView
)

func loadAPIKeys(path string, databases ...*database.Store) (map[string]apiKey, error) {
	keys := make(map[string]apiKey)
	found, err := loadState(configuredDatabase(databases), path, &keys)
	return identitycore.LoadedAPIKeys(keys, found, err)
}

func (store *profileStore) commitAPIKey(secret string, key apiKey) error {
	store.mu.Lock()
	defer store.mu.Unlock()
	keys := cloneAPIKeys(store.apiKeys)
	keys[sessionKey(secret)] = key
	if err := store.persist(store.apiFile, keys); err != nil {
		return err
	}
	store.apiKeys = keys
	return nil
}

func (store *profileStore) createAPIKey(profile viewerProfile, name, value string) (string, error) {
	name = strings.TrimSpace(name)
	scopes, err := apiScopes(value)
	if err != nil {
		return "", err
	}
	if name == "" || len(name) > 80 || len(scopes) == 0 {
		return "", errors.New("API key name and scopes are required")
	}
	return identitycore.IssueAPIKey(profile.ID, name, scopes, false, store.commitAPIKey)
}

func apiScopes(value string) ([]string, error) {
	return identitycore.ParseAPIScopes(value)
}

func cloneAPIKeys(keys map[string]apiKey) map[string]apiKey {
	return identitycore.CloneAPIKeys(keys)
}

func (store *profileStore) apiKeyViews() []apiKeyView {
	store.mu.RLock()
	defer store.mu.RUnlock()
	return identitycore.APIKeyViews(store.apiKeys)
}

func (store *profileStore) revokeAPIKey(id string) error {
	store.mu.Lock()
	defer store.mu.Unlock()
	if _, found := store.apiKeys[id]; !found {
		return errors.New("API key was not found")
	}
	keys := cloneAPIKeys(store.apiKeys)
	delete(keys, id)
	if err := store.persist(store.apiFile, keys); err != nil {
		return err
	}
	store.apiKeys = keys
	return nil
}

func profileAllowsAPI(profile viewerProfile, pattern string) bool {
	return profile.AllowsAPI(pattern, identitycore.APIRoutes{SessionOnly: apiSessionOnlyRoutes, Library: apiLibraryRoutes, Stream: apiStreamRoutes})
}

var apiSessionOnlyRoutes = routeSet(
	"GET /api/v1/backup", "POST /api/v1/management-access", "DELETE /api/v1/management-access", "POST /api/v1/management-access/devices", "DELETE /api/v1/management-access/devices",
	"DELETE /api/v1/session", "GET /api/v1/me/oidc/link", "DELETE /api/v1/me/oidc", "GET /api/v1/me/saml/link", "DELETE /api/v1/me/saml",
	"POST /api/v1/me/mfa/setup", "PUT /api/v1/me/mfa", "DELETE /api/v1/me/mfa",
	"GET /api/v1/passkeys", "DELETE /api/v1/passkeys/{id}", "POST /api/v1/passkeys/register/begin", "POST /api/v1/passkeys/register/finish",
)

var apiLibraryRoutes = routeSet(
	"GET /api/v1", "GET /api/v1/openapi.json", "GET /api/v1/library", "GET /api/v1/subtitle-library", "GET /art/{id}",
)

var apiStreamRoutes = routeSet(
	"GET /api/v1/events",
)

func routeSet(patterns ...string) map[string]bool {
	return identitycore.RouteSet(patterns...)
}
