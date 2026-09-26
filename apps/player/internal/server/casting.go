package server

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/MikeO7/kinosail/packages/casting"
	"github.com/MikeO7/kinosail/packages/identitycore"
)

type castTrack struct {
	ID       int    `json:"id"`
	URL      string `json:"url"`
	Label    string `json:"label"`
	Language string `json:"language"`
	Default  bool   `json:"default"`
	source   string
}

type castSession struct {
	Tracks                     []castTrack `json:"tracks"`
	ID                         string      `json:"id"`
	URL                        string      `json:"url"`
	ContentType                string      `json:"contentType"`
	Title                      string      `json:"title"`
	Position                   float64     `json:"position"`
	Duration                   float64     `json:"duration"`
	ExpiresAt                  time.Time   `json:"expiresAt"`
	Protocol                   string      `json:"protocol"`
	DeviceID                   string      `json:"deviceId,omitempty"`
	DeviceName                 string      `json:"deviceName,omitempty"`
	itemID, profileID, version string
	revision                   uint64
	apiKey                     bool
	scopes                     []string
	proof                      [32]byte
	plan                       PlaybackPlan
}

type castService struct {
	mu        sync.Mutex
	sessions  map[string]castSession
	auth      *authentication
	index     *libraryIndex
	probe     *mediaProbe
	hls       *hlsManager
	settings  *settingsStore
	base      string
	renderers *casting.Renderers
	now       func() time.Time
}

type castTicketKey struct{}

func registerCasting(mux *http.ServeMux, auth *authentication, index *libraryIndex, probe *mediaProbe, hls *hlsManager, settings *settingsStore, base string) {
	service := &castService{sessions: make(map[string]castSession), auth: auth, index: index, probe: probe, hls: hls, settings: settings, base: strings.TrimRight(base, "/"), renderers: casting.NewRenderers(), now: time.Now}
	mux.HandleFunc("POST /api/v1/items/{id}/cast", service.startHTTP)
	mux.HandleFunc("DELETE /api/v1/cast/sessions/{id}", service.endHTTP)
	mux.HandleFunc("POST /api/v1/cast/devices/scan", service.scanHTTP)
	mux.HandleFunc("GET /api/v1/cast/config", service.configHTTP)
	mux.HandleFunc("GET /api/v1/cast/sessions/{id}", service.statusHTTP)
	mux.HandleFunc("POST /api/v1/cast/sessions/{id}/commands", service.commandHTTP)
	mux.HandleFunc("GET /cast/{id}/media", service.mediaHTTP)
	mux.HandleFunc("GET /cast/{id}/subtitles/{track}", service.mediaHTTP)
	mux.HandleFunc("OPTIONS /cast/{id}/subtitles/{track}", service.mediaHTTP)
	mux.HandleFunc("GET /cast/{id}/hls/{file...}", service.mediaHTTP)
	mux.HandleFunc("OPTIONS /cast/{id}/media", service.mediaHTTP)
	mux.HandleFunc("OPTIONS /cast/{id}/hls/{file...}", service.mediaHTTP)
}

func (service *castService) startHTTP(writer http.ResponseWriter, request *http.Request) {
	var input casting.Start
	if !readJSON(writer, request, &input) {
		return
	}
	session, err := service.start(request, request.PathValue("id"), input)
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, errCastEntropy) {
			status = http.StatusServiceUnavailable
		}
		apiError(writer, err, status)
		return
	}
	writer.Header().Set("Cache-Control", "no-store")
	writeJSON(writer, session, http.StatusCreated)
}

// start is the shared boundary: authorization and complete input validation precede a grant or device command.
func (service *castService) start(request *http.Request, itemID string, input casting.Start) (castSession, error) {
	if invalidCastRequest(request, itemID, input) {
		return castSession{}, casting.ErrInvalid
	}
	item, viewer, deviceName, err := service.authorizeCast(request, itemID, input)
	if err != nil {
		return castSession{}, err
	}
	media := service.probe.inspect(request.Context(), item)
	position, err := castSourcePosition(input, media.Duration)
	if err != nil {
		return castSession{}, err
	}
	facts := mediaFactsFor(item, media)
	plan, err := service.planCast(facts, viewer, item.Kind, input.Protocol)
	if err != nil {
		return castSession{}, err
	}
	id, err := castRandom(16)
	if err != nil {
		return castSession{}, err
	}
	token, err := castRandom(32)
	if err != nil {
		return castSession{}, err
	}
	lifetime := time.Duration(min(max(media.Duration+3600, 3600), 24*3600)) * time.Second
	session := castSession{ID: id, Title: item.Title, Position: position, Duration: media.Duration, ExpiresAt: service.now().Add(lifetime), Protocol: input.Protocol, DeviceID: input.DeviceID, DeviceName: deviceName, itemID: item.ID, profileID: viewer.ID, revision: viewer.Revision, apiKey: viewer.APIKey, scopes: append([]string(nil), viewer.Scopes...), proof: sha256.Sum256([]byte(token)), plan: plan, version: facts.FileVersion}
	if err := service.setCastURL(&session, item.Path, token); err != nil {
		return castSession{}, err
	}
	service.attachCastTracks(&session, item, media, token)
	if err := service.rememberCast(session); err != nil {
		return castSession{}, err
	}
	if input.Protocol == "dlna" {
		if err := service.renderers.Load(request.Context(), input.DeviceID, session.URL, session.ContentType, session.Title, session.Position); err != nil {
			service.remove(id, viewer.ID)
			return castSession{}, err
		}
	}
	return session, nil
}

var errCastEntropy = errors.New("Could not create a cast session. Try again.")

func castRandom(size int) (string, error) {
	value := make([]byte, size)
	if _, err := io.ReadFull(rand.Reader, value); err != nil {
		return "", errCastEntropy
	}
	return hex.EncodeToString(value), nil
}

func (service *castService) remove(id, viewer string) {
	service.mu.Lock()
	defer service.mu.Unlock()
	if session, ok := service.sessions[id]; ok && session.profileID == viewer {
		delete(service.sessions, id)
	}
}

func (service *castService) session(id string) (castSession, bool) {
	if !casting.ValidSessionID(id) {
		return castSession{}, false
	}
	service.mu.Lock()
	defer service.mu.Unlock()
	session, ok := service.sessions[id]
	if ok && !session.ExpiresAt.After(service.now()) {
		delete(service.sessions, id)
		ok = false
	}
	return session, ok
}

func (service *castService) viewer(session castSession) (viewerProfile, bool) {
	if session.profileID == "local-owner" {
		return viewerProfile{ID: "local-owner", Owner: true}, !service.auth.required && !service.auth.profiles.hasProfiles()
	}
	service.auth.profiles.mu.RLock()
	defer service.auth.profiles.mu.RUnlock()
	viewer, found := identitycore.FindProfile(service.auth.profiles.profiles, session.profileID)
	viewer.APIKey, viewer.Scopes = session.apiKey, session.scopes
	return viewer, found && service.auth.profiles.err == nil && viewer.Revision == session.revision && !viewer.Disabled && !viewer.SCIMDeleted && viewer.Allowed(false, service.now()) && viewer.Permits("stream", true)
}

func (service *castService) setCastURL(session *castSession, itemPath, token string) error {
	session.ContentType = map[string]string{".mp4": "video/mp4", ".m4v": "video/mp4", ".mov": "video/quicktime", ".mp3": "audio/mpeg", ".aac": "audio/aac", ".m4a": "audio/mp4", ".wav": "audio/wav"}[strings.ToLower(filepath.Ext(itemPath))]
	if session.ContentType == "" && session.plan.Mode == "direct" {
		return errors.New("This file format cannot play directly on the TV") //nolint:staticcheck // This complete sentence is displayed directly in the TV picker.
	}
	path := "/cast/" + session.ID + "/media"
	if session.plan.Mode != "direct" {
		path = "/cast/" + session.ID + "/hls/index.m3u8"
		session.ContentType = "application/vnd.apple.mpegurl"
	}
	session.URL = service.base + path + "?ticket=" + token
	return nil
}

func invalidCastRequest(request *http.Request, itemID string, input casting.Start) bool {
	return input.Validate(itemID) != nil || request.URL.RawQuery != "" || publicInternetRequest(request)
}
