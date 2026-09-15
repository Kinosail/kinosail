package updatecontrol

import (
	"errors"
	"mime"
	"net/http"
	"net/url"
	"slices"

	"github.com/MikeO7/kinosail/packages/httpguard"
)

// HTTPConfig connects shared update handlers to app response presentation.
type HTTPConfig struct {
	ReadJSON func(http.ResponseWriter, *http.Request, any) bool
	JSON     func(http.ResponseWriter, any, int)
	APIError func(http.ResponseWriter, error, int)
	WebError func(http.ResponseWriter, *http.Request, string, int)
}

// HTTPHandlers owns the versioned and form update request behavior.
type HTTPHandlers struct {
	checker *Checker
	config  HTTPConfig
}

// NewHTTPHandlers validates presentation adapters before serving requests.
func NewHTTPHandlers(checker *Checker, config HTTPConfig) (*HTTPHandlers, error) {
	if checker == nil || config.ReadJSON == nil || config.JSON == nil || config.APIError == nil || config.WebError == nil {
		return nil, errors.New("invalid update HTTP configuration")
	}
	return &HTTPHandlers{checker: checker, config: config}, nil
}

// View writes the current update state.
func (handlers *HTTPHandlers) View(writer http.ResponseWriter, _ *http.Request) {
	handlers.config.JSON(writer, handlers.checker.View(), http.StatusOK)
}

// Request asks the installed manager to install the checked release.
func (handlers *HTTPHandlers) Request(writer http.ResponseWriter, request *http.Request) {
	if !httpguard.EmptyMutationRequest(writer, request) {
		handlers.config.APIError(writer, errors.New("update request must be empty"), http.StatusBadRequest)
		return
	}
	status, err := handlers.checker.RequestAvailable()
	if err != nil {
		handlers.config.APIError(writer, errors.New("could not request an update"), http.StatusInternalServerError)
		return
	}
	if status.State != "available" || status.Manager.RequestID == "" {
		handlers.config.APIError(writer, errors.New("no update is available"), http.StatusConflict)
		return
	}
	handlers.config.JSON(writer, map[string]string{"status": "requested", "requestId": status.Manager.RequestID, "targetVersion": status.LatestVersion}, http.StatusAccepted)
}

// Preference changes the automatic update preference through JSON.
func (handlers *HTTPHandlers) Preference(writer http.ResponseWriter, request *http.Request) {
	var input struct {
		Automatic *bool `json:"automatic"`
	}
	if !handlers.config.ReadJSON(writer, request, &input) {
		return
	}
	if input.Automatic == nil {
		handlers.config.APIError(writer, errors.New("automatic is required"), http.StatusBadRequest)
		return
	}
	if err := handlers.checker.SetAutomatic(*input.Automatic); err != nil {
		handlers.config.APIError(writer, errors.New("could not save update preference"), http.StatusInternalServerError)
		return
	}
	handlers.config.JSON(writer, handlers.checker.View(), http.StatusOK)
}

// Check refreshes update state through an empty JSON request.
func (handlers *HTTPHandlers) Check(writer http.ResponseWriter, request *http.Request) {
	if !handlers.config.ReadJSON(writer, request, &struct{}{}) {
		return
	}
	status := handlers.checker.Check(request.Context())
	code := http.StatusOK
	if status.State == "unavailable" {
		code = http.StatusServiceUnavailable
	}
	handlers.config.JSON(writer, status, code)
}

// SavePreference changes the automatic preference through a strict form.
func (handlers *HTTPHandlers) SavePreference(redirect string) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		if !formEncoded(request) || request.URL.RawQuery != "" || request.ParseForm() != nil || !onlyFormKeys(request.PostForm, "mode") {
			handlers.config.WebError(writer, request, "invalid update preference", http.StatusBadRequest)
			return
		}
		mode, ok := oneValue(request.PostForm, "mode", 9)
		if !ok || mode != "manual" && mode != "automatic" {
			handlers.config.WebError(writer, request, "update preference must be manual or automatic", http.StatusBadRequest)
			return
		}
		if err := handlers.checker.SetAutomatic(mode == "automatic"); err != nil {
			handlers.config.WebError(writer, request, "could not save update preference", http.StatusInternalServerError)
			return
		}
		http.Redirect(writer, request, redirect, http.StatusSeeOther)
	}
}

// CheckAndRequest refreshes and requests through a strict empty form.
func (handlers *HTTPHandlers) CheckAndRequest(redirect string) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		if !formEncoded(request) || request.URL.RawQuery != "" || request.ParseForm() != nil || len(request.PostForm) != 0 {
			handlers.config.WebError(writer, request, "invalid update check", http.StatusBadRequest)
			return
		}
		if _, err := handlers.checker.CheckAndRequest(request.Context()); err != nil {
			handlers.config.WebError(writer, request, "could not request an update", http.StatusInternalServerError)
			return
		}
		http.Redirect(writer, request, redirect, http.StatusSeeOther)
	}
}

// ParseMode validates the optional setup update mode and its allowed peers.
func ParseMode(request *http.Request, required bool, allowed ...string) (bool, error) {
	if err := request.ParseForm(); err != nil {
		return false, errors.New("invalid update preference")
	}
	for key := range request.Form {
		if key != "updateMode" && !slices.Contains(allowed, key) {
			return false, errors.New("invalid update preference")
		}
	}
	values, found := request.Form["updateMode"]
	if !found {
		if required {
			return false, errors.New("update preference is required")
		}
		return true, nil
	}
	if len(values) != 1 || values[0] != "manual" && values[0] != "automatic" {
		return false, errors.New("update preference must be manual or automatic")
	}
	return values[0] == "automatic", nil
}

// StatusText returns the user-facing release check summary.
func StatusText(status Status) string {
	switch status.State {
	case "available":
		return status.LatestVersion + " is available"
	case "current":
		return "Kinosail is current"
	case "unavailable":
		return "The last check could not reach GitHub"
	case "no-release":
		return "No public Kinosail release is available yet"
	default:
		return "No update check has run"
	}
}

// ManagerStatusText returns the user-facing installed manager summary.
func ManagerStatusText(status View) string {
	switch status.Status {
	case "requested":
		return "The installed update manager has received the update request."
	case "checking", "available", "installing":
		return "The installed update manager is " + status.Status + "."
	case "current":
		return "The installed update manager reports that Kinosail is current."
	case "failed", "rolled-back":
		return "The installed update manager reports: " + status.Message
	default:
		return "No installed update adapter has reported yet."
	}
}

func formEncoded(request *http.Request) bool {
	mediaType, _, err := mime.ParseMediaType(request.Header.Get("Content-Type"))
	return err == nil && mediaType == "application/x-www-form-urlencoded"
}

func onlyFormKeys(form url.Values, keys ...string) bool {
	for key := range form {
		if !slices.Contains(keys, key) {
			return false
		}
	}
	return true
}

func oneValue(values url.Values, key string, limit int) (string, bool) {
	value := ""
	if entries := values[key]; len(entries) == 1 {
		value = entries[0]
	}
	return value, value != "" && len(value) <= limit && len(values[key]) == 1
}
