package server

import (
	"errors"
	"math"
	"net/http"
	"strconv"
	"time"
)

func (store *settingsStore) sessionTimeouts() (time.Duration, time.Duration) {
	store.mu.RLock()
	defer store.mu.RUnlock()
	inactive, absolute := sessionTimeoutHours(store.value)
	return time.Duration(inactive * float64(time.Hour)), time.Duration(absolute * float64(time.Hour))
}

func sessionTimeoutHours(settings installationSettings) (float64, float64) {
	inactive, absolute := settings.SessionInactiveHours, settings.SessionAbsoluteHours
	if inactive == 0 {
		inactive = float64(defaultSessionInactive) / float64(time.Hour)
	}
	if absolute == 0 {
		absolute = float64(defaultSessionAbsolute) / float64(time.Hour)
	}
	return inactive, absolute
}

func (store *settingsStore) setSessionTimeouts(inactive, absolute float64) error {
	if !validSessionTimeouts(inactive, absolute) {
		return errors.New("session timeouts must be 0.25–8760 inactive hours and 4–8760 absolute hours, with inactivity no longer than the absolute limit")
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	settings := store.value
	settings.SessionInactiveHours, settings.SessionAbsoluteHours = inactive, absolute
	if err := store.save(settings); err != nil {
		return err
	}
	store.value = settings
	return nil
}

func validSessionTimeouts(inactive, absolute float64) bool {
	finite := func(value float64) bool { return !math.IsNaN(value) && !math.IsInf(value, 0) }
	return finite(inactive) && finite(absolute) && inactive >= .25 && inactive <= 365*24 && absolute >= 4 && absolute <= 365*24 && inactive <= absolute
}

func saveSessionTimeouts(settings *settingsStore) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		if !strictSingleValueForm(request, "inactiveHours", "absoluteHours") {
			localizedError(writer, request, "session timeouts are invalid", http.StatusBadRequest)
			return
		}
		inactive, inactiveErr := strconv.ParseFloat(request.PostForm.Get("inactiveHours"), 64)
		absolute, absoluteErr := strconv.ParseFloat(request.PostForm.Get("absoluteHours"), 64)
		if inactiveErr != nil || absoluteErr != nil || settings.setSessionTimeouts(inactive, absolute) != nil {
			localizedError(writer, request, "session timeouts are invalid", http.StatusBadRequest)
			return
		}
		http.Redirect(writer, request, "/settings#security", http.StatusSeeOther)
	}
}

func apiSessionTimeouts(settings *settingsStore) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		var input struct{ InactiveHours, AbsoluteHours float64 }
		if !readJSON(writer, request, &input) {
			return
		}
		if err := settings.setSessionTimeouts(input.InactiveHours, input.AbsoluteHours); err != nil {
			apiError(writer, err, http.StatusBadRequest)
			return
		}
		writeJSON(writer, map[string]string{"status": "saved"}, http.StatusOK)
	}
}
