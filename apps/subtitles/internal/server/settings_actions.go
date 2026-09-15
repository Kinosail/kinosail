package server

import (
	"errors"
	"io"
	"net/http"
	"path/filepath"

	"github.com/MikeO7/kinosail-subtitles/internal/backup"
	settingsops "github.com/MikeO7/kinosail/packages/settings"
)

func saveMFARequirement(auth *authentication) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		revoked, err := auth.setRequireMFA(request.FormValue("required") == "true")
		if err != nil {
			if errors.Is(err, errManagedSetting) {
				localizedError(writer, request, err.Error(), http.StatusConflict)
				return
			}
			localizedError(writer, request, "could not save MFA requirement", http.StatusInternalServerError)
			return
		}
		if revoked {
			cookie := sessionCookie("") //nolint:gosec // sessionCookie always sets Secure, HttpOnly, and Strict SameSite.
			cookie.MaxAge = -1
			http.SetCookie(writer, cookie)
			http.Redirect(writer, request, "/login", http.StatusSeeOther)
			return
		}
		http.Redirect(writer, request, "/settings#security", http.StatusSeeOther)
	}
}

func apiMFARequirement(auth *authentication) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		var input struct {
			Required bool `json:"required"`
		}
		if !readJSON(writer, request, &input) {
			return
		}
		revoked, err := auth.setRequireMFA(input.Required)
		if err != nil {
			if errors.Is(err, errManagedSetting) {
				apiError(writer, err, http.StatusConflict)
				return
			}
			apiError(writer, errors.New("could not save MFA requirement"), http.StatusInternalServerError)
			return
		}
		writeJSON(writer, map[string]any{"required": input.Required, "sessionsRevoked": revoked}, http.StatusOK)
	}
}

func createAPIKey(profiles *profileStore) http.HandlerFunc {
	return settingsops.CreateAPIKey(
		func(request *http.Request, name, scopes string) (string, error) {
			return profiles.createAPIKey(currentViewer(request), name, scopes)
		},
		func(writer http.ResponseWriter, request *http.Request, name, secret string) error {
			return apiKeySecretView.Execute(writer, request, struct{ Name, Secret string }{name, secret})
		}, localizedError,
	)
}

func revokeAPIKey(profiles *profileStore) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		if err := profiles.revokeAPIKey(request.FormValue("id")); err != nil {
			localizedError(writer, request, err.Error(), http.StatusBadRequest)
			return
		}
		http.Redirect(writer, request, "/settings", http.StatusSeeOther)
	}
}

func revokeDevice(profiles *profileStore) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		if err := profiles.revokeDevice(request.FormValue("id")); err != nil {
			localizedError(writer, request, err.Error(), http.StatusBadRequest)
			return
		}
		http.Redirect(writer, request, "/settings", http.StatusSeeOther)
	}
}

func saveSubtitleLanguage(settings *settingsStore, destination string) http.HandlerFunc {
	return saveSubtitleLanguageForm(settings, destination, false)
}

func savePrimarySubtitleLanguage(settings *settingsStore, destination string) http.HandlerFunc {
	return saveSubtitleLanguageForm(settings, destination, true)
}

func invalidSubtitleLanguageAction(primary, actionPresent bool, action string, preferencePresent bool) bool {
	if !actionPresent {
		return false
	}
	if action == "" {
		return true
	}
	if primary {
		return true
	}
	return preferencePresent
}

func saveSubtitleLanguageForm(settings *settingsStore, destination string, primary bool) http.HandlerFunc { //nolint:cyclop,gocritic // Form variants share one validation and mutation boundary.
	return func(writer http.ResponseWriter, request *http.Request) {
		request.Body = http.MaxBytesReader(writer, request.Body, 4096)
		if !formEncoded(request) || request.URL.RawQuery != "" || request.ParseForm() != nil || !onlyFormKeys(request.PostForm, "language", "action", "preference") || len(request.PostForm["language"]) != 1 || len(request.PostForm["action"]) > 1 || len(request.PostForm["preference"]) > 1 {
			localizedError(writer, request, "subtitle language request is invalid", http.StatusBadRequest)
			return
		}
		var err error
		action, actionPresent := request.PostForm.Get("action"), len(request.PostForm["action"]) == 1
		preference, preferencePresent := request.PostForm.Get("preference"), len(request.PostForm["preference"]) == 1
		switch {
		case invalidSubtitleLanguageAction(primary, actionPresent, action, preferencePresent):
			err = errors.New("subtitle language action is invalid")
		case actionPresent:
			err = settings.changeSubtitleLanguages(action, request.PostForm.Get("language"))
		case preferencePresent:
			err = settings.setSubtitlePlan(request.PostForm.Get("language"), preference)
		default:
			err = settings.setPrimarySubtitleLanguage(request.PostForm.Get("language"))
		}
		if err != nil {
			localizedError(writer, request, err.Error(), http.StatusBadRequest)
			return
		}
		http.Redirect(writer, request, destination, http.StatusSeeOther)
	}
}

func downloadBackup(settings *settingsStore) http.HandlerFunc {
	return settingsops.DownloadBackup(settings.file != "", func(writer io.Writer) error {
		return backup.Write(writer, filepath.Dir(settings.file))
	}, localizedError)
}

func revokeSessions(profiles *profileStore) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		if err := profiles.revokeOtherSessions(request); err != nil {
			localizedError(writer, request, err.Error(), http.StatusInternalServerError)
			return
		}
		http.Redirect(writer, request, "/settings", http.StatusSeeOther)
	}
}

func clearCache(hls *hlsManager) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		if err := hls.cacheOps.Clear(); err != nil {
			localizedError(writer, request, err.Error(), http.StatusConflict)
			return
		}
		http.Redirect(writer, request, "/settings", http.StatusSeeOther)
	}
}

func saveScanFrequency(settings *settingsStore, index *libraryIndex, destination string) http.HandlerFunc {
	return settingsops.SaveScanFrequency(settings.setScanFrequency, index.SetFrequency, destination, localizedError)
}

func saveTranscoder(settings *settingsStore) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		if err := settings.setTranscoder(request.FormValue("quality"), request.FormValue("codec"), request.FormValue("accelerator"), request.FormValue("toneMap") == "true"); err != nil {
			localizedError(writer, request, err.Error(), http.StatusBadRequest)
			return
		}
		http.Redirect(writer, request, "/settings", http.StatusSeeOther)
	}
}

func savePlayback(settings *settingsStore) http.HandlerFunc {
	return settingsops.SavePlayback(func(value settingsops.Playback) error {
		return settings.setPlayback(value.PlaybackMode, value.Autoplay, value.Subtitles, value.AutoSkip)
	}, localizedError)
}

func saveServerName(settings *settingsStore) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		if err := settings.setName(request.FormValue("name")); err != nil {
			localizedError(writer, request, err.Error(), http.StatusBadRequest)
			return
		}
		http.Redirect(writer, request, "/settings", http.StatusSeeOther)
	}
}
