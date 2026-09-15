package settings

import (
	"bytes"
	"errors"
	"io"
	"mime"
	"net/http"
	"strings"

	"github.com/MikeO7/kinosail/packages/identitycore"
)

// WriteError preserves one app's localized error response.
type WriteError func(http.ResponseWriter, *http.Request, string, int)

// SaveOnboardingAPI handles the shared onboarding preference mutation.
func SaveOnboardingAPI(change func(bool) error, read func(http.ResponseWriter, *http.Request, any) bool, write func(http.ResponseWriter, any, int), writeError func(http.ResponseWriter, error, int)) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		var input struct {
			Enabled *bool `json:"enabled"`
		}
		if !read(writer, request, &input) {
			return
		}
		if input.Enabled == nil {
			writeError(writer, errors.New("onboarding enabled is required"), http.StatusBadRequest)
			return
		}
		if err := change(*input.Enabled); err != nil {
			writeError(writer, err, http.StatusInternalServerError)
			return
		}
		write(writer, map[string]string{"status": "saved"}, http.StatusOK)
	}
}

// SavePlayback handles the shared playback settings form.
func SavePlayback(change func(Playback) error, writeError WriteError) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		if err := decodeForm(writer, request, 4096, map[string]int{"mode": 1, "autoplay": 1, "subtitles": 1, "markers": 5}); err != nil {
			writeError(writer, request, "playback mode is invalid", http.StatusBadRequest)
			return
		}
		autoplay := request.PostForm.Get("autoplay")
		if autoplay != "" && autoplay != "true" {
			writeError(writer, request, "playback mode is invalid", http.StatusBadRequest)
			return
		}
		input := Playback{PlaybackMode: request.PostForm.Get("mode"), Subtitles: request.PostForm.Get("subtitles"), Autoplay: autoplay == "true", AutoSkip: append([]string{}, request.PostForm["markers"]...)}
		if err := change(input); err != nil {
			writeError(writer, request, err.Error(), http.StatusBadRequest)
			return
		}
		http.Redirect(writer, request, "/settings", http.StatusSeeOther)
	}
}

// SaveScanFrequency handles the shared scan schedule form.
func SaveScanFrequency(change func(string) error, apply func(string), destination string, writeError WriteError) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		if err := decodeForm(writer, request, 1024, map[string]int{"frequency": 1}); err != nil {
			writeError(writer, request, "scan frequency is invalid", http.StatusBadRequest)
			return
		}
		frequency := request.PostForm.Get("frequency")
		if err := change(frequency); err != nil {
			writeError(writer, request, err.Error(), http.StatusBadRequest)
			return
		}
		apply(frequency)
		http.Redirect(writer, request, destination, http.StatusSeeOther)
	}
}

// CreateAPIKey handles the shared owner API-key form and one-time secret response.
func CreateAPIKey(create func(*http.Request, string, string) (string, error), render func(http.ResponseWriter, *http.Request, string, string) error, writeError WriteError) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		limits := map[string]int{"name": 1, "scopes": 1, "write": 1, "stream": 1, "download": 1, "admin": 1}
		if err := decodeForm(writer, request, 2048, limits); err != nil {
			writeError(writer, request, "API key name and scopes are required", http.StatusBadRequest)
			return
		}
		name := request.PostForm.Get("name")
		scopes := request.PostForm.Get("scopes") + "," + request.PostForm.Get("write") + "," + request.PostForm.Get("stream") + "," + request.PostForm.Get("download") + "," + request.PostForm.Get("admin")
		if strings.TrimSpace(name) == "" || len(name) > 80 {
			writeError(writer, request, "API key name and scopes are required", http.StatusBadRequest)
			return
		}
		parsedScopes, err := identitycore.ParseAPIScopes(scopes)
		if err != nil {
			writeError(writer, request, err.Error(), http.StatusBadRequest)
			return
		}
		if len(parsedScopes) == 0 {
			writeError(writer, request, "API key name and scopes are required", http.StatusBadRequest)
			return
		}
		secret, err := create(request, name, scopes)
		if err != nil {
			writeError(writer, request, err.Error(), http.StatusBadRequest)
			return
		}
		writer.Header().Set("Content-Type", "text/html; charset=utf-8")
		writer.Header().Set("Cache-Control", "no-store")
		writer.WriteHeader(http.StatusCreated)
		if render(writer, request, name, secret) != nil {
			writeError(writer, request, "API key view failed", http.StatusInternalServerError)
		}
	}
}

// DownloadBackup writes one portable application state archive.
func DownloadBackup(available bool, archive func(io.Writer) error, writeError WriteError) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		var data bytes.Buffer
		if !available || archive(&data) != nil {
			writeError(writer, request, "backup is unavailable", http.StatusServiceUnavailable)
			return
		}
		writer.Header().Set("Content-Type", "application/gzip")
		writer.Header().Set("Content-Disposition", `attachment; filename="kinosail-backup.tar.gz"`)
		_, _ = writer.Write(data.Bytes())
	}
}

// ShowBackups renders one application's localized backup status page.
func ShowBackups[T any](status func() T, render func(http.ResponseWriter, *http.Request, any) error, writeError WriteError) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := render(writer, request, status()); err != nil {
			writeError(writer, request, err.Error(), http.StatusInternalServerError)
		}
	}
}

// VerifyBackup runs verification and returns to the backup page on success.
func VerifyBackup(verify func() error, writeError WriteError) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		if err := verify(); err != nil {
			writeError(writer, request, err.Error(), http.StatusServiceUnavailable)
			return
		}
		http.Redirect(writer, request, "/settings/backups", http.StatusSeeOther)
	}
}

func decodeForm(writer http.ResponseWriter, request *http.Request, maximum int64, limits map[string]int) error {
	mediaType, _, err := mime.ParseMediaType(request.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/x-www-form-urlencoded" || request.URL.RawQuery != "" {
		return errors.New("invalid form request")
	}
	request.Body = http.MaxBytesReader(writer, request.Body, maximum)
	if request.ParseForm() != nil {
		return errors.New("invalid form request")
	}
	for key, values := range request.PostForm {
		limit, ok := limits[key]
		if !ok || len(values) > limit {
			return errors.New("invalid form request")
		}
	}
	return nil
}
