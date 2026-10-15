package jellyfincompat

import (
	"net/http"

	"github.com/MikeO7/kinosail/packages/library"
)

type PlaybackState struct {
	ItemID                string `json:"ItemId"`
	PositionTicks         int64  `json:"PositionTicks"`
	PlaybackPositionTicks int64  `json:"PlaybackPositionTicks"`
	Played                *bool  `json:"Played"`
	PlaySessionID         string `json:"PlaySessionId"`
	EventSequence         uint64 `json:"EventSequence"`
}

type UserState struct {
	Seconds float64
	Watched bool
}

type OperationError struct {
	Status  int
	Message string
}

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

func SaveFavorite(found bool, save func() error) *OperationError {
	if !found {
		return &OperationError{Status: http.StatusNotFound}
	}
	if err := save(); err != nil {
		return &OperationError{Status: http.StatusInternalServerError, Message: "could not save favorite"}
	}
	return nil
}

func SavePlayed(found bool, save func() error) *OperationError {
	if !found {
		return &OperationError{Status: http.StatusNotFound}
	}
	if err := save(); err != nil {
		return &OperationError{Status: http.StatusInternalServerError, Message: "could not save playback state"}
	}
	return nil
}
