package homeassistant

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"mime"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/MikeO7/kinosail/packages/httpguard"
)

// ErrorFunc writes one app-localized error.
type ErrorFunc func(http.ResponseWriter, *http.Request, string, int)

// Approval contains the app-branded approval view state.
type Approval struct{ ProfileName, RequestID, Destination string }

// Register installs the complete Home Assistant route set.
func (integration *Integration[P]) Register(mux *http.ServeMux, owner func(http.Handler) http.Handler, approval func(http.ResponseWriter, *http.Request, Approval) error) {
	mux.Handle("GET /api/v1/home-assistant", integration.available(http.HandlerFunc(integration.probeHTTP)))
	mux.Handle("POST /api/v1/home-assistant/token", integration.available(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) { integration.tokenHTTP(writer, request) })))
	mux.Handle("POST /api/v1/home-assistant/pair", integration.available(http.HandlerFunc(integration.pairHTTP)))
	mux.Handle("GET /home-assistant/authorize", owner(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		integration.authorizeHTTP(writer, request, approval)
	})))
	mux.Handle("POST /home-assistant/authorize", owner(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		integration.authorizeHTTP(writer, request, approval)
	})))
	mux.Handle("POST /api/v1/home-assistant/pairings", integration.available(owner(http.HandlerFunc(integration.offerHTTP))))
	mux.Handle("GET /api/v1/home-assistant/library", integration.available(integration.control(http.HandlerFunc(integration.libraryHTTP))))
	mux.Handle("POST /api/v1/home-assistant/playback/{id}", integration.available(integration.control(http.HandlerFunc(integration.playbackHTTP))))
	mux.Handle("GET /api/v1/home-assistant/players", integration.available(integration.control(http.HandlerFunc(integration.playersHTTP))))
	mux.Handle("PUT /api/v1/home-assistant/players/{id}", integration.available(http.HandlerFunc(integration.playerStateHTTP)))
	mux.Handle("POST /api/v1/home-assistant/players/{id}/commands", integration.available(integration.control(http.HandlerFunc(integration.commandHTTP))))
	mux.Handle("GET /home-assistant/media/{id}", integration.available(http.HandlerFunc(integration.mediaHTTP)))
}

// Gate hides all Home Assistant routes while the feature is disabled.
func Gate(enabled func() bool, next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if (strings.HasPrefix(request.URL.Path, "/api/v1/home-assistant") || strings.HasPrefix(request.URL.Path, "/home-assistant/")) && !enabled() {
			if strings.HasPrefix(request.URL.Path, "/api/") {
				apiNotFound(writer)
			} else {
				http.NotFound(writer, request)
			}
			return
		}
		next.ServeHTTP(writer, request)
	})
}

// SettingHandler changes the feature through the versioned HTTP API.
func (integration *Integration[P]) SettingHandler() http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		var input struct {
			Enabled *bool `json:"enabled"`
		}
		if !readJSON(writer, request, &input) {
			return
		}
		if input.Enabled == nil {
			apiError(writer, errors.New("enabled is required"), http.StatusBadRequest)
			return
		}
		if err := integration.SetEnabled(*input.Enabled); err != nil {
			apiError(writer, err, http.StatusBadRequest)
			return
		}
		writeJSON(writer, map[string]string{"status": "saved"}, http.StatusOK)
	}
}

// SaveSetting changes the feature through one strict web form.
func (integration *Integration[P]) SaveSetting(destination string, writeError ErrorFunc) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		if err := httpguard.DecodeForm(writer, request, 1024, "enabled"); err != nil {
			writeError(writer, request, "Home Assistant setting is invalid", http.StatusBadRequest)
			return
		}
		values := request.PostForm["enabled"]
		if len(values) == 1 && values[0] != "true" {
			writeError(writer, request, "Home Assistant setting is invalid", http.StatusBadRequest)
			return
		}
		if err := integration.SetEnabled(len(values) == 1); err != nil {
			writeError(writer, request, err.Error(), http.StatusBadRequest)
			return
		}
		http.Redirect(writer, request, destination, http.StatusSeeOther)
	}
}

// PairingHandler renders one app-branded manual pairing page.
func (integration *Integration[P]) PairingHandler(render func(http.ResponseWriter, *http.Request, string) error, writeError ErrorFunc, notFound func(http.ResponseWriter, *http.Request)) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		if !integration.Enabled() {
			notFound(writer, request)
			return
		}
		code, err := integration.offer(integration.config.CurrentProfile(request))
		if err != nil {
			writeError(writer, request, err.Error(), http.StatusServiceUnavailable)
			return
		}
		writer.Header().Set("Content-Type", "text/html; charset=utf-8")
		writer.Header().Set("Cache-Control", "no-store")
		if err := render(writer, request, code); err != nil {
			writeError(writer, request, "Home Assistant pairing failed", http.StatusInternalServerError)
		}
	}
}

func (integration *Integration[P]) available(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if !integration.Enabled() {
			apiNotFound(writer)
			return
		}
		next.ServeHTTP(writer, request)
	})
}

func (integration *Integration[P]) control(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		profile := integration.config.CurrentProfile(request)
		if profile.Owner || profile.APIKey && slices.Contains(profile.Scopes, "home-assistant") {
			next.ServeHTTP(writer, request)
			return
		}
		apiError(writer, errors.New("Owner access required"), http.StatusForbidden) //nolint:staticcheck // Preserve the public error.
	})
}

func (integration *Integration[P]) probeHTTP(writer http.ResponseWriter, _ *http.Request) {
	server := integration.config.Server()
	writeJSON(writer, map[string]any{"name": server.Name, "serverId": server.ID, "apiVersion": "v1"}, http.StatusOK)
}

func (integration *Integration[P]) offerHTTP(writer http.ResponseWriter, request *http.Request) {
	if !readJSON(writer, request, &struct{}{}) {
		return
	}
	code, err := integration.offer(integration.config.CurrentProfile(request))
	if err != nil {
		apiError(writer, err, http.StatusServiceUnavailable)
		return
	}
	writeJSON(writer, map[string]any{"code": code, "expiresIn": int(pairingTTL / time.Second)}, http.StatusCreated)
}

func (integration *Integration[P]) pairHTTP(writer http.ResponseWriter, request *http.Request) {
	var input struct{ Code, Name string }
	if !readJSON(writer, request, &input) {
		return
	}
	token, err := integration.pair(input.Code, input.Name)
	if err != nil {
		apiError(writer, err, http.StatusBadRequest)
		return
	}
	server := integration.config.Server()
	writeJSON(writer, map[string]any{"token": token, "serverId": server.ID, "name": server.Name}, http.StatusCreated)
}

func (integration *Integration[P]) libraryHTTP(writer http.ResponseWriter, request *http.Request) {
	page, err := integration.config.Browse(request)
	if err != nil {
		apiError(writer, err, http.StatusBadRequest)
		return
	}
	writeJSON(writer, map[string]any{"items": page.Items, "view": page.View, "total": page.Total, "offset": page.Offset, "limit": page.Limit}, http.StatusOK)
}

func (integration *Integration[P]) playbackHTTP(writer http.ResponseWriter, request *http.Request) {
	item, found := integration.config.VisibleItem(request, request.PathValue("id"))
	if !found {
		apiNotFound(writer)
		return
	}
	expires := integration.now().Add(mediaTTL).Unix()
	signature := integration.sign(item.ID, expires)
	mimeType := mime.TypeByExtension(item.Path)
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}
	writeJSON(writer, map[string]any{"url": "/home-assistant/media/" + item.ID + "?expires=" + strconv.FormatInt(expires, 10) + "&signature=" + url.QueryEscape(signature), "mimeType": mimeType, "expiresIn": int(mediaTTL / time.Second)}, http.StatusOK)
}

func (integration *Integration[P]) sign(id string, expires int64) string {
	mac := hmac.New(sha256.New, integration.secret[:])
	_, _ = fmt.Fprintf(mac, "%s\n%d", id, expires)
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func (integration *Integration[P]) mediaHTTP(writer http.ResponseWriter, request *http.Request) {
	query := request.URL.Query()
	expiresValue, expiresOK := oneValue(query, "expires", 20)
	signature, signatureOK := oneValue(query, "signature", 64)
	expires, err := strconv.ParseInt(expiresValue, 10, 64)
	want := integration.sign(request.PathValue("id"), expires)
	now := integration.now()
	if !onlyValues(query, "expires", "signature") || !expiresOK || !signatureOK || err != nil || expires < now.Unix() || expires > now.Add(mediaTTL+time.Minute).Unix() || !hmac.Equal([]byte(want), []byte(signature)) {
		http.NotFound(writer, request)
		return
	}
	item, found := integration.config.FindItem(request.PathValue("id"))
	if !found || !integration.config.SafePath(item.Path) {
		http.NotFound(writer, request)
		return
	}
	http.ServeFile(writer, request, item.Path)
}

func (integration *Integration[P]) playersHTTP(writer http.ResponseWriter, _ *http.Request) {
	writeJSON(writer, map[string]any{"players": integration.playersSnapshot()}, http.StatusOK)
}

func (integration *Integration[P]) playerStateHTTP(writer http.ResponseWriter, request *http.Request) {
	id := request.PathValue("id")
	var state Player
	if !playerID.MatchString(id) || !readJSON(writer, request, &state) {
		if !playerID.MatchString(id) {
			apiError(writer, errors.New("Home Assistant player ID is invalid"), http.StatusBadRequest) //nolint:staticcheck // Preserve the public error.
		}
		return
	}
	command, err := integration.updatePlayer(id, state, integration.config.CurrentProfile(request).ID)
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, errPlayerLimit) {
			status = http.StatusTooManyRequests
		}
		apiError(writer, err, status)
		return
	}
	if command == nil {
		writeJSON(writer, map[string]any{"command": nil}, http.StatusOK)
		return
	}
	writeJSON(writer, command, http.StatusOK)
}

func (integration *Integration[P]) commandHTTP(writer http.ResponseWriter, request *http.Request) {
	id := request.PathValue("id")
	var command Command
	if !playerID.MatchString(id) || !readJSON(writer, request, &command) {
		if !playerID.MatchString(id) {
			apiError(writer, errors.New("Home Assistant player ID is invalid"), http.StatusBadRequest) //nolint:staticcheck // Preserve the public error.
		}
		return
	}
	_, found, err := integration.queueCommand(id, command)
	if err != nil {
		apiError(writer, err, http.StatusBadRequest)
		return
	}
	if !found {
		apiNotFound(writer)
		return
	}
	writeJSON(writer, map[string]string{"status": "queued"}, http.StatusAccepted)
}

func readJSON(writer http.ResponseWriter, request *http.Request, target any) bool {
	if err := httpguard.DecodeRequestJSON(writer, request, target); err != nil {
		apiError(writer, err, http.StatusBadRequest)
		return false
	}
	return true
}

func writeJSON(writer http.ResponseWriter, value any, status int) {
	writer.Header().Set("Content-Type", "application/json")
	writer.Header().Set("Cache-Control", "no-store")
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(value)
}

func apiError(writer http.ResponseWriter, err error, status int) {
	writeJSON(writer, map[string]string{"error": err.Error()}, status)
}

func apiNotFound(writer http.ResponseWriter) {
	apiError(writer, errors.New("not found"), http.StatusNotFound)
}
