package server

import (
	"errors"
	"net/http"
	"path/filepath"

	"github.com/MikeO7/kinosail-subtitles/internal/configuration"
)

const subSourceConfigurationKey = "integrations.subsource"

var subSourceConfigurationKeys = []string{"integrations.subsource.api_key", "integrations.subsource.personal_use"}

func registerSubSourceConfiguration(mux *http.ServeMux, auth *authentication, settings *settingsStore) {
	mux.Handle("POST /settings/subtitles/subsource", auth.owner(saveSubSourceConfiguration(settings, false)))
	mux.Handle("POST /settings/subtitles/subsource/reset", auth.owner(saveSubSourceConfiguration(settings, true)))
}

func (store *settingsStore) changeSubSourceConfiguration(apiKey string, reset bool) error {
	store.mu.Lock()
	defer store.mu.Unlock()
	if store.file == "" {
		return errors.New("configuration storage is unavailable")
	}
	for _, key := range subSourceConfigurationKeys {
		if store.config.Managed(key) {
			return errors.New(key + " is managed by " + string(store.config.Source(key)))
		}
	}
	dataDir := filepath.Dir(store.file)
	if reset {
		if err := configuration.DeleteSubSource(dataDir); err != nil {
			return err
		}
		for _, key := range subSourceConfigurationKeys {
			store.config.UpdateGUI(key, "", true)
		}
		store.refreshSubtitleProviderLocked(subSourceConfigurationKey)
		return nil
	}
	if apiKey == "" {
		apiKey = store.config.String("integrations.subsource.api_key")
	}
	if err := configuration.SetSubSource(dataDir, apiKey); err != nil {
		return err
	}
	store.config.UpdateGUI("integrations.subsource.api_key", apiKey, false)
	store.config.UpdateGUI("integrations.subsource.personal_use", "true", false)
	store.refreshSubtitleProviderLocked(subSourceConfigurationKey)
	return nil
}

func saveSubSourceConfiguration(settings *settingsStore, reset bool) http.HandlerFunc { //nolint:cyclop,gocognit // Reset and accepted-form paths share one mutation boundary.
	return func(writer http.ResponseWriter, request *http.Request) {
		if reset {
			if !emptyMutationRequest(writer, request) {
				localizedError(writer, request, "invalid SubSource form", http.StatusBadRequest)
				return
			}
		} else {
			if !formEncoded(request) || request.URL.RawQuery != "" || request.ParseForm() != nil || !onlyFormKeys(request.PostForm, "apiKey", "personalUse") {
				localizedError(writer, request, "invalid SubSource form", http.StatusBadRequest)
				return
			}
			apiKey, keyOK := firstBounded(request.PostForm["apiKey"], 4096, false)
			personalUse, accepted := oneValue(request.PostForm, "personalUse", 4)
			if !keyOK || !accepted || personalUse != "true" {
				localizedError(writer, request, "SubSource personal-use terms must be accepted", http.StatusBadRequest)
				return
			}
			if err := settings.changeSubSourceConfiguration(apiKey, false); err != nil {
				localizedError(writer, request, err.Error(), http.StatusConflict)
				return
			}
			http.Redirect(writer, request, "/settings#provider", http.StatusSeeOther)
			return
		}
		if err := settings.changeSubSourceConfiguration("", true); err != nil {
			localizedError(writer, request, err.Error(), http.StatusConflict)
			return
		}
		http.Redirect(writer, request, "/settings#provider", http.StatusSeeOther)
	}
}

func apiChangeSubSourceConfiguration(writer http.ResponseWriter, request *http.Request, settings *settingsStore, reset bool) {
	var input struct {
		APIKey      string `json:"apiKey"`
		PersonalUse bool   `json:"personalUse"`
	}
	if !reset && !readJSON(writer, request, &input) {
		return
	}
	if !reset && !input.PersonalUse {
		apiError(writer, errors.New("SubSource personal-use terms must be accepted"), http.StatusBadRequest)
		return
	}
	if err := settings.changeSubSourceConfiguration(input.APIKey, reset); err != nil {
		apiError(writer, err, http.StatusConflict)
		return
	}
	writeJSON(writer, map[string]any{"status": "saved", "restartRequired": false}, http.StatusAccepted)
}
