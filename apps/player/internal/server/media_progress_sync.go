package server

import (
	"errors"
	"math"
	"net/http"
	"strings"
	"time"
	"unicode"

	"github.com/MikeO7/kinosail/packages/catalog"
)

type mediaProgressSnapshot struct {
	Seconds  float64 `json:"seconds"`
	Watched  bool    `json:"watched"`
	Session  string  `json:"session"`
	Revision uint64  `json:"revision"`
}

func validSyncSnapshot(value mediaProgressSnapshot, required bool) bool {
	return !math.IsNaN(value.Seconds) && !math.IsInf(value.Seconds, 0) && value.Seconds >= 0 && value.Seconds <= 31_536_000 &&
		len(value.Session) <= 128 && !strings.ContainsFunc(value.Session, unicode.IsControl) && value.Revision <= 9_007_199_254_740_991 &&
		(!required || (value.Session != "" && value.Revision > 0))
}

func syncMediaProgress(index *libraryIndex, progress *progressStore) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		serveMediaProgress(writer, request, index, progress)
	}
}

func serveMediaProgress(writer http.ResponseWriter, request *http.Request, index *libraryIndex, progress *progressStore) {
	if !matchingProgressViewer(writer, request) {
		return
	}
	item, found := visibleItem(request, index, request.PathValue("id"))
	if !found {
		apiNotFound(writer)
		return
	}
	input, valid := readProgressSynchronization(writer, request)
	if !valid {
		return
	}
	timeline, err := timelineFromPlaybackToken(input.PlaybackToken)
	if err != nil {
		apiError(writer, errors.New("invalid playback token"), http.StatusBadRequest)
		return
	}
	position := input.Progress.Seconds
	if len(timeline.Omitted) > 0 {
		position = timeline.SourceTime(position)
	}
	expected, update := *input.Expected, *input.Progress
	key := currentViewer(request).ID + ":" + item.ID
	_, state, accepted, err := progress.Update(key, synchronizedProgress(position, expected, update))
	if err != nil {
		apiError(writer, errors.New("progress could not be saved"), http.StatusInternalServerError)
		return
	}
	if !accepted && (state.Session != update.Session || state.Revision < update.Revision) {
		writeJSON(writer, map[string]any{"error": "Progress changed on another device.", "progress": state}, http.StatusConflict)
		return
	}
	writeJSON(writer, state, http.StatusOK)
}

func (value *mediaProgressSnapshot) UnmarshalJSON(raw []byte) error {
	type plain mediaProgressSnapshot
	return decodeMediaFields(raw, (*plain)(value), []string{"seconds", "watched", "session", "revision"})
}

func synchronizedProgress(position float64, expected, update mediaProgressSnapshot) func(playbackState) (playbackState, bool, error) {
	return func(state playbackState) (playbackState, bool, error) {
		// Retrying a request after a lost response must not create a conflict or
		// rewind a newer event from this same playback session.
		if state.Session == update.Session && state.Revision >= update.Revision {
			return state, false, nil
		}
		if state.Session != update.Session && (state.Session != expected.Session || state.Revision != expected.Revision || state.Seconds != expected.Seconds || state.Watched != expected.Watched) {
			return state, false, nil
		}
		return catalog.ProgressRevision(position, &update.Watched, update.Session, update.Revision, time.Now())(state)
	}
}

func matchingProgressViewer(writer http.ResponseWriter, request *http.Request) bool {
	values, supplied := request.Header["X-Kinosail-Viewer-Profile"]
	if supplied && (len(values) != 1 || values[0] == "" || len(values[0]) > 128 || values[0] != currentViewer(request).ID) {
		apiError(writer, errors.New("Viewer Profile changed. Open the Server again to sync progress."), http.StatusForbidden) //nolint:staticcheck // This complete recovery instruction is displayed directly to the viewer.
		return false
	}
	return true
}

type mediaProgressSynchronization struct {
	Progress      *mediaProgressSnapshot `json:"progress"`
	Expected      *mediaProgressSnapshot `json:"expected"`
	PlaybackToken string                 `json:"playbackToken"`
}

func readProgressSynchronization(writer http.ResponseWriter, request *http.Request) (mediaProgressSynchronization, bool) {
	var input mediaProgressSynchronization
	if !readMediaJSON(writer, request, &input) {
		return input, false
	}
	if input.Progress == nil || input.Expected == nil || !validSyncSnapshot(*input.Progress, true) || !validSyncSnapshot(*input.Expected, false) {
		apiError(writer, errors.New("invalid progress synchronization"), http.StatusBadRequest)
		return input, false
	}
	return input, true
}
