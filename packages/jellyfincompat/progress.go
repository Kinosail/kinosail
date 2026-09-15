package jellyfincompat

import (
	"net/http"

	"github.com/MikeO7/kinosail/packages/library"
)

// PlaybackState is the Jellyfin playback and user-data input contract.
type PlaybackState struct {
	ItemID                string `json:"ItemId"`
	PositionTicks         int64  `json:"PositionTicks"`
	PlaybackPositionTicks int64  `json:"PlaybackPositionTicks"`
	Played                *bool  `json:"Played"`
	PlaySessionID         string `json:"PlaySessionId"`
	EventSequence         uint64 `json:"EventSequence"`
}

// UserState is the app-owned playback state needed by Jellyfin feeds.
type UserState struct {
	Seconds float64
	Watched bool
}

// OperationError describes one Player-canonical Jellyfin error response.
type OperationError struct {
	Status  int
	Message string
}

// ProjectUserDataDTO maps app-owned playback time before projection when allowed.
func ProjectUserDataDTO(item library.Item, state UserState, favorite bool, presentation func(float64) float64) map[string]any {
	if state.Seconds > 0 && item.Kind == "video" && presentation != nil {
		state.Seconds = presentation(state.Seconds)
	}
	id := ID(item.ID)
	return map[string]any{
		"ItemId": id, "Key": id, "PlaybackPositionTicks": int64(state.Seconds * 1e7),
		"Played": state.Watched, "IsFavorite": favorite,
	}
}

// SaveProgress applies shared Jellyfin validation and stale-write semantics.
func SaveProgress(found bool, ticks int64, save func(float64) (bool, error), invalid func(error) bool) *OperationError {
	if !found || ticks < 0 {
		return &OperationError{Status: http.StatusBadRequest, Message: "invalid playback state"}
	}
	accepted, err := save(float64(ticks) / 1e7)
	if err != nil {
		if invalid(err) {
			return &OperationError{Status: http.StatusBadRequest, Message: "invalid playback state"}
		}
		return &OperationError{Status: http.StatusInternalServerError, Message: "could not save playback state"}
	}
	if !accepted {
		return &OperationError{Status: http.StatusConflict, Message: "playback state is stale"}
	}
	return nil
}

// SaveFavorite applies shared Jellyfin item and persistence semantics.
func SaveFavorite(found bool, save func() error) *OperationError {
	if !found {
		return &OperationError{Status: http.StatusNotFound}
	}
	if err := save(); err != nil {
		return &OperationError{Status: http.StatusInternalServerError, Message: "could not save favorite"}
	}
	return nil
}

// SavePlayed applies shared Jellyfin item and playback persistence semantics.
func SavePlayed(found bool, save func() error) *OperationError {
	if !found {
		return &OperationError{Status: http.StatusNotFound}
	}
	if err := save(); err != nil {
		return &OperationError{Status: http.StatusInternalServerError, Message: "could not save playback state"}
	}
	return nil
}
