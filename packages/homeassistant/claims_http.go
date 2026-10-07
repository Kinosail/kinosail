package homeassistant

import (
	"bytes"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/MikeO7/kinosail/packages/httpguard"
)

const playerClaimHeader = "X-Kinosail-Player-Claim"

func requestPlayerClaim(request *http.Request) (string, bool) {
	values := request.Header.Values(playerClaimHeader)
	if len(values) == 0 {
		return "", true
	}
	if len(values) != 1 || len(values[0]) < 20 || !playerID.MatchString(values[0]) {
		return "", false
	}
	return values[0], true
}

func playerFailure(writer http.ResponseWriter, request *http.Request, operation, id string, err error, status int) {
	failure, level := "invalid", slog.LevelWarn
	switch {
	case errors.Is(err, errPlayerDisabled):
		failure, level = "disabled", slog.LevelDebug
	case errors.Is(err, errPlayerOwnership):
		failure = "ownership"
	case errors.Is(err, errPlayerOccupied):
		failure, level = "occupied", slog.LevelDebug
	case errors.Is(err, errPlayerLimit):
		failure = "capacity"
	case status >= http.StatusInternalServerError:
		failure = "unavailable"
	}
	fields := []any{"operation", operation, "failure", failure, "status", status}
	if playerID.MatchString(id) {
		fields = append(fields, "target_id", id)
	}
	if requestID := writer.Header().Get("X-Request-ID"); playerID.MatchString(requestID) {
		fields = append(fields, "request_id", requestID)
	}
	slog.Log(request.Context(), level, "Home Assistant player request rejected", fields...)
	apiError(writer, err, status)
}

func readPlayerJSON(writer http.ResponseWriter, request *http.Request, input any, operation, id string) bool {
	if err := httpguard.DecodeRequestJSON(writer, request, input); err != nil {
		playerFailure(writer, request, operation, id, err, http.StatusBadRequest)
		return false
	}
	return true
}

func (integration *Integration[P]) playerClaimHTTP(writer http.ResponseWriter, request *http.Request) {
	var input struct {
		ID json.RawMessage `json:"id,omitempty"`
	}
	if !readPlayerJSON(writer, request, &input, "claim", "") {
		return
	}
	id := ""
	if value := bytes.TrimSpace(input.ID); len(value) != 0 && (value[0] != '"' || json.Unmarshal(value, &id) != nil) || id != "" && !playerID.MatchString(id) {
		playerFailure(writer, request, "claim", "", errors.New("player ID is invalid"), http.StatusBadRequest)
		return
	}
	claim, retry, err := integration.claimPlayer(id, integration.config.CurrentProfile(request).ID)
	if err != nil {
		status := http.StatusServiceUnavailable
		switch {
		case errors.Is(err, errPlayerDisabled):
			status = http.StatusNotFound
		case errors.Is(err, errPlayerOccupied):
			status = http.StatusConflict
			writer.Header().Set("Retry-After", strconv.Itoa(retry))
		case errors.Is(err, errPlayerOwnership):
			status = http.StatusForbidden
		case errors.Is(err, errPlayerLimit):
			status = http.StatusTooManyRequests
		}
		playerFailure(writer, request, "claim", id, err, status)
		return
	}
	writeJSON(writer, claim, http.StatusCreated)
}

func (integration *Integration[P]) playerReleaseHTTP(writer http.ResponseWriter, request *http.Request) {
	id := request.PathValue("id")
	if !playerID.MatchString(id) || !readPlayerJSON(writer, request, &struct{}{}, "release", id) {
		if !playerID.MatchString(id) {
			playerFailure(writer, request, "release", "", errors.New("player ID is invalid"), http.StatusBadRequest)
		}
		return
	}
	claim, valid := requestPlayerClaim(request)
	if !valid || claim == "" {
		playerFailure(writer, request, "release", id, errPlayerOwnership, http.StatusForbidden)
		return
	}
	if err := integration.releasePlayer(id, integration.config.CurrentProfile(request).ID, claim); err != nil {
		playerFailure(writer, request, "release", id, err, http.StatusForbidden)
		return
	}
	writer.Header().Set("Cache-Control", "no-store")
	writer.WriteHeader(http.StatusNoContent)
}

func (integration *Integration[P]) playerStateHTTP(writer http.ResponseWriter, request *http.Request) {
	id := request.PathValue("id")
	var state Player
	if !playerID.MatchString(id) || !readPlayerJSON(writer, request, &state, "state", id) {
		if !playerID.MatchString(id) {
			playerFailure(writer, request, "state", "", errors.New("Home Assistant player ID is invalid"), http.StatusBadRequest) //nolint:staticcheck // Preserve the public error.
		}
		return
	}
	claim, valid := requestPlayerClaim(request)
	if !valid {
		playerFailure(writer, request, "state", id, errPlayerOwnership, http.StatusForbidden)
		return
	}
	command, err := integration.updatePlayer(id, state, integration.config.CurrentProfile(request).ID, claim)
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, errPlayerLimit) {
			status = http.StatusTooManyRequests
		} else if errors.Is(err, errPlayerOwnership) {
			status = http.StatusForbidden
		}
		playerFailure(writer, request, "state", id, err, status)
		return
	}
	if command == nil {
		writeJSON(writer, map[string]any{"command": nil}, http.StatusOK)
		return
	}
	writeJSON(writer, command, http.StatusOK)
}
