package server

import (
	"errors"
	"github.com/MikeO7/kinosail/packages/httpguard"
	"net/http"
	"time"
)

func publicSessionTimeoutHours(settings installationSettings) (float64, float64) {
	if settings.PublicSessionInactiveHours != 0 && settings.PublicSessionAbsoluteHours != 0 {
		return settings.PublicSessionInactiveHours, settings.PublicSessionAbsoluteHours
	}
	// Absent fields preserve legacy validation, even when inactivity exceeds the cap.
	inactive, absolute := sessionTimeoutHours(settings)
	return inactive, min(absolute, 8)
}

func (store *settingsStore) publicSessionTimeouts() (time.Duration, time.Duration) {
	store.mu.RLock()
	defer store.mu.RUnlock()
	inactive, absolute := publicSessionTimeoutHours(store.value)
	return time.Duration(inactive * float64(time.Hour)), time.Duration(absolute * float64(time.Hour))
}

func (store *settingsStore) publicSessionLifetime() time.Duration {
	store.mu.RLock()
	defer store.mu.RUnlock()
	return publicSessionLifetime(store.value)
}

func publicSessionLifetime(settings installationSettings) time.Duration {
	if settings.PublicSessionAbsoluteHours != 0 {
		return time.Duration(settings.PublicSessionAbsoluteHours * float64(time.Hour))
	}
	// Legacy issuance and its cookie always used eight hours, independently of validation.
	return 8 * time.Hour
}

func (auth *authentication) publicAuthenticationMaximumAge() time.Duration {
	if auth.settings.snapshot().PublicSessionAbsoluteHours == 0 {
		return publicSessionMaximumAge
	}
	_, absolute := auth.settings.publicSessionTimeouts()
	return absolute
}

func resetPublicSessionTimeouts(settings *settingsStore, api bool) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		if !httpguard.EmptyMutationRequest(writer, request) {
			if api {
				apiError(writer, errors.New("reset requires an empty request"), http.StatusBadRequest)
			} else {
				localizedError(writer, request, "reset requires an empty request", http.StatusBadRequest)
			}
			return
		}
		if err := settings.resetPublicSessionTimeouts(); err != nil {
			timeoutSettingsFailure(request)
			if api {
				apiError(writer, errors.New("could not reset public session timeouts"), http.StatusInternalServerError)
			} else {
				localizedError(writer, request, "could not reset public session timeouts", http.StatusInternalServerError)
			}
			return
		}
		if api {
			writeJSON(writer, map[string]string{"status": "reset"}, http.StatusOK)
		} else {
			http.Redirect(writer, request, "/settings#security", http.StatusSeeOther)
		}
	}
}
