package server

import (
	"errors"
	"net/http"

	"github.com/MikeO7/kinosail/packages/trustedhttps"
)

var errJellyfinTrustedHTTPSRequired = errors.New("trusted HTTPS is required before Jellyfin apps can connect")

func (store *settingsStore) setJellyfinCompatibility(enabled bool) error {
	store.mu.Lock()
	defer store.mu.Unlock()
	if err := store.editableLocked("integrations.jellyfin.enabled"); err != nil {
		return err
	}
	if enabled && !trustedHTTPSConfigured(store.config.String("tls.duckdns")) {
		return errJellyfinTrustedHTTPSRequired
	}
	settings := store.value
	settings.JellyfinCompatibility = enabled
	if err := store.save(settings); err != nil {
		return err
	}
	store.value = settings
	return nil
}

func (store *settingsStore) jellyfinCompatibility() bool {
	store.mu.RLock()
	defer store.mu.RUnlock()
	return store.value.JellyfinCompatibility && trustedHTTPSConfigured(store.config.String("tls.duckdns"))
}

func trustedHTTPSConfigured(raw string) bool {
	config, err := trustedhttps.Parse(raw)
	return err == nil && config != (trustedhttps.Config{})
}

func jellyfinCompatibility(settings *settingsStore, next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if jellyfinPath(request.URL.Path) && !settings.jellyfinCompatibility() {
			http.NotFound(writer, request)
			return
		}
		next.ServeHTTP(writer, request)
	})
}

func saveJellyfinCompatibility(settings *settingsStore, redirect string) http.HandlerFunc { //nolint:cyclop,gocognit // Compatibility settings use one validated web mutation path.
	return func(writer http.ResponseWriter, request *http.Request) {
		if err := request.ParseForm(); err != nil {
			localizedError(writer, request, "invalid Jellyfin form", http.StatusBadRequest)
			return
		}
		for key, values := range request.PostForm {
			if len(values) != 1 || key != "enabled" && key != "_csrf" {
				localizedError(writer, request, "invalid Jellyfin form", http.StatusBadRequest)
				return
			}
		}
		enabled := request.PostForm.Get("enabled") == "true"
		if values := request.PostForm["enabled"]; len(values) == 1 && values[0] != "true" {
			localizedError(writer, request, "invalid Jellyfin choice", http.StatusBadRequest)
			return
		}
		if err := settings.setJellyfinCompatibility(enabled); err != nil {
			status := http.StatusBadRequest
			if errors.Is(err, errManagedSetting) || errors.Is(err, errJellyfinTrustedHTTPSRequired) {
				status = http.StatusConflict
			}
			localizedError(writer, request, err.Error(), status)
			return
		}
		http.Redirect(writer, request, redirect, http.StatusSeeOther)
	}
}
