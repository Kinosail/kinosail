package server

import (
	"errors"
	"math"
	"mime"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/MikeO7/kinosail/packages/httpguard"
)

const remotePlayerTTL = 30 * time.Second

var (
	remotePlayerID = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
	remoteItemID   = regexp.MustCompile(`^[A-Za-z0-9_-]{1,128}$`)
)

type remotePlayerState struct {
	ID       string  `json:"id"`
	Name     string  `json:"name"`
	Title    string  `json:"title"`
	Artist   string  `json:"artist"`
	ItemID   string  `json:"itemId"`
	State    string  `json:"state"`
	Position float64 `json:"position"`
	Duration float64 `json:"duration"`
	Audio    bool    `json:"audio"`
}

type remotePlayerInput struct {
	Name     string  `json:"name"`
	Title    string  `json:"title"`
	Artist   string  `json:"artist"`
	ItemID   string  `json:"itemId"`
	State    string  `json:"state"`
	Position float64 `json:"position"`
	Duration float64 `json:"duration"`
	Audio    bool    `json:"audio"`
}

type remotePlayerCommand struct {
	Command  string   `json:"command"`
	ItemID   string   `json:"itemId"`
	Position *float64 `json:"position,omitempty"`
}

type remotePlayerRecord struct {
	remotePlayerState
	profile string
	seen    time.Time
	queue   []remotePlayerCommand
}

type remotePlayers struct {
	mu      sync.Mutex
	records map[string]remotePlayerRecord
	now     func() time.Time
}

func newRemotePlayers() *remotePlayers {
	return &remotePlayers{records: make(map[string]remotePlayerRecord), now: time.Now}
}

func (service *remotePlayers) prune() {
	for id, record := range service.records {
		if service.now().Sub(record.seen) > remotePlayerTTL {
			delete(service.records, id)
		}
	}
}

func validRemoteState(state remotePlayerState) bool {
	if !validRemoteLabels(state) || !validRemoteTimes(state.Position, state.Duration) {
		return false
	}
	return validRemoteActivity(state)
}

func validRemoteActivity(state remotePlayerState) bool {
	switch state.State {
	case "idle":
		return state.Title == "" && state.Artist == "" && state.ItemID == "" && state.Position == 0 && state.Duration == 0 && !state.Audio
	case "playing", "paused", "buffering":
		return strings.TrimSpace(state.Title) != "" && remoteItemID.MatchString(state.ItemID)
	default:
		return false
	}
}

func validRemoteLabels(state remotePlayerState) bool {
	return validRemoteText(state.Name, 80, true) && validRemoteText(state.Title, 256, false) &&
		validRemoteText(state.Artist, 128, false)
}

func validRemoteTimes(position, duration float64) bool {
	return !math.IsNaN(position) && !math.IsInf(position, 0) && !math.IsNaN(duration) && !math.IsInf(duration, 0) &&
		position >= 0 && duration >= 0 && duration <= 1e9 && position <= duration
}

func validRemoteText(value string, maximum int, required bool) bool {
	if !utf8.ValidString(value) || len(value) > maximum || required && strings.TrimSpace(value) == "" {
		return false
	}
	for _, character := range value {
		if unicode.IsControl(character) {
			return false
		}
	}
	return true
}

func validRemoteCommand(command remotePlayerCommand, state remotePlayerState) bool {
	if state.State == "idle" || command.ItemID != state.ItemID {
		return false
	}
	switch command.Command {
	case "play", "pause", "backward", "forward":
		return command.Position == nil
	case "previous", "next":
		return command.Position == nil && state.Audio
	case "seek":
		return validRemoteSeek(command.Position, state.Duration)
	default:
		return false
	}
}

func validRemoteSeek(position *float64, duration float64) bool {
	return position != nil && !math.IsNaN(*position) && !math.IsInf(*position, 0) && *position >= 0 && *position <= duration
}

func remotePlayerPathID(request *http.Request) (string, bool) {
	id := request.PathValue("id")
	return id, remotePlayerID.MatchString(id) && request.URL.RawQuery == ""
}

func (service *remotePlayers) register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/remote-players", service.listHTTP)
	mux.HandleFunc("PUT /api/v1/remote-players/{id}", service.updateHTTP)
	mux.HandleFunc("POST /api/v1/remote-players/{id}/commands", service.commandHTTP)
}

func decodeRemoteJSON(writer http.ResponseWriter, request *http.Request, target any) bool {
	mediaType, _, err := mime.ParseMediaType(request.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" || httpguard.DecodeUniqueJSON(http.MaxBytesReader(writer, request.Body, 4096), 4096, target) != nil {
		apiError(writer, errors.New("invalid JSON request"), http.StatusBadRequest)
		return false
	}
	return true
}

func (service *remotePlayers) listHTTP(writer http.ResponseWriter, request *http.Request) {
	if request.URL.RawQuery != "" {
		apiError(writer, errors.New("invalid query"), http.StatusBadRequest)
		return
	}
	service.mu.Lock()
	service.prune()
	players := make([]remotePlayerState, 0, len(service.records))
	for _, record := range service.records {
		if record.profile == currentViewer(request).ID {
			players = append(players, record.remotePlayerState)
		}
	}
	service.mu.Unlock()
	writeJSON(writer, map[string]any{"players": players}, http.StatusOK)
}

func (service *remotePlayers) updateHTTP(writer http.ResponseWriter, request *http.Request) {
	id, valid := remotePlayerPathID(request)
	if !valid {
		apiError(writer, errors.New("invalid player ID"), http.StatusBadRequest)
		return
	}
	var input remotePlayerInput
	if !decodeRemoteJSON(writer, request, &input) {
		return
	}
	state := remotePlayerState{id, input.Name, input.Title, input.Artist, input.ItemID, input.State, input.Position, input.Duration, input.Audio}
	if !validRemoteState(state) {
		apiError(writer, errors.New("invalid player state"), http.StatusBadRequest)
		return
	}
	service.mu.Lock()
	service.prune()
	record, found := service.records[id]
	if found && record.profile != currentViewer(request).ID {
		service.mu.Unlock()
		apiError(writer, errors.New("player unavailable"), http.StatusNotFound)
		return
	}
	if !found && len(service.records) >= 64 {
		service.mu.Unlock()
		apiError(writer, errors.New("too many players"), http.StatusTooManyRequests)
		return
	}
	if record.ItemID != state.ItemID {
		record.queue = nil
	}
	record.remotePlayerState, record.profile, record.seen = state, currentViewer(request).ID, service.now()
	var command *remotePlayerCommand
	if len(record.queue) > 0 {
		next := record.queue[0]
		command = &next
		record.queue = record.queue[1:]
	}
	service.records[id] = record
	service.mu.Unlock()
	writeJSON(writer, map[string]any{"command": command}, http.StatusOK)
}

func (service *remotePlayers) commandHTTP(writer http.ResponseWriter, request *http.Request) {
	id, valid := remotePlayerPathID(request)
	if !valid {
		apiError(writer, errors.New("invalid player ID"), http.StatusBadRequest)
		return
	}
	var command remotePlayerCommand
	if !decodeRemoteJSON(writer, request, &command) {
		return
	}
	service.mu.Lock()
	service.prune()
	record, found := service.records[id]
	if !found || record.profile != currentViewer(request).ID {
		service.mu.Unlock()
		apiError(writer, errors.New("player unavailable"), http.StatusNotFound)
		return
	}
	if !validRemoteCommand(command, record.remotePlayerState) || len(record.queue) >= 8 {
		service.mu.Unlock()
		apiError(writer, errors.New("invalid player command"), http.StatusBadRequest)
		return
	}
	record.queue = append(record.queue, command)
	service.records[id] = record
	service.mu.Unlock()
	writeJSON(writer, map[string]string{"status": "queued"}, http.StatusAccepted)
}
