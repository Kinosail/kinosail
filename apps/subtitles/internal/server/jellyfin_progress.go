package server

import (
	"errors"
	"net/http"
	"strings"
	"time"

	sharedjellyfin "github.com/MikeO7/kinosail/packages/jellyfincompat"
	"github.com/MikeO7/kinosail/packages/library"
	"github.com/MikeO7/kinosail/packages/playback"
)

type jellyfinPlaybackState = sharedjellyfin.PlaybackState

func (api *jellyfinAPI) playbackProgress(writer http.ResponseWriter, request *http.Request) {
	var state jellyfinPlaybackState
	if !readJellyfinJSON(request, &state) {
		http.Error(writer, "invalid playback state", http.StatusBadRequest)
		return
	}
	stopped := strings.HasSuffix(request.URL.Path, "/Stopped")
	state.PositionTicks = api.sourcePositionTicks(state.PlaySessionID, jellyfinRawID(state.ItemID), state.PositionTicks)
	api.saveJellyfinProgress(writer, request, state.ItemID, state.PositionTicks, state.Played, state.PlaySessionID, state.EventSequence, stopped)
}

func (api *jellyfinAPI) userData(writer http.ResponseWriter, request *http.Request) {
	item, found := visibleItem(request, api.index, jellyfinRawID(request.PathValue("id")))
	if !found {
		http.NotFound(writer, request)
		return
	}
	if request.Method != http.MethodPost {
		jellyfinJSON(writer, api.userDataDTO(request, item))
		return
	}
	if writeJellyfinOperationError(writer, request, api.updateJellyfinUserData(request, item)) {
		return
	}
	jellyfinJSON(writer, api.userDataDTO(request, item))
}

func (api *jellyfinAPI) updateJellyfinUserData(request *http.Request, item library.Item) *sharedjellyfin.OperationError {
	var state jellyfinPlaybackState
	if !readJellyfinJSON(request, &state) {
		return &sharedjellyfin.OperationError{Status: http.StatusBadRequest, Message: "invalid playback state"}
	}
	ticks := api.sourcePositionTicks(state.PlaySessionID, item.ID, state.PlaybackPositionTicks)
	return sharedjellyfin.SaveProgress(true, ticks, func(seconds float64) (bool, error) {
		return api.progress.SetRevision(request, item.ID, seconds, state.Played, state.PlaySessionID, state.EventSequence)
	}, func(err error) bool { return errors.Is(err, errInvalidProgressState) })
}

func (api *jellyfinAPI) sourcePositionTicks(playSessionID, itemID string, ticks int64) int64 {
	value, found := playback.LoadJellyfinPlaySession(&api.plays, playSessionID, time.Now())
	session, valid := value.(jellyfinPlaySession)
	if !found || !valid || session.itemID != itemID || session.plan.MarkerMode != "server" || time.Now().After(session.expires) {
		return ticks
	}
	return int64(session.plan.Timeline.SourceTime(float64(ticks)/1e7) * 1e7)
}

func (api *jellyfinAPI) played(writer http.ResponseWriter, request *http.Request) {
	watched := request.Method == http.MethodPost
	item, found := visibleItem(request, api.index, jellyfinRawID(request.PathValue("id")))
	seconds := 0.0
	if found {
		seconds = api.progress.Get(request, item.ID).Seconds
	}
	operationErr := sharedjellyfin.SavePlayed(found, func() error { return api.progress.Set(request, item.ID, seconds, &watched) })
	if writeJellyfinOperationError(writer, request, operationErr) {
		return
	}
	jellyfinJSON(writer, api.userDataDTO(request, item))
}

func (api *jellyfinAPI) favorite(writer http.ResponseWriter, request *http.Request) { //nolint:contextcheck // The callback passes the captured request context to storage.
	item, found := visibleItem(request, api.index, jellyfinRawID(request.PathValue("id")))
	operationErr := sharedjellyfin.SaveFavorite(found, func() error { //nolint:contextcheck // The callback passes the captured request context to storage.
		return api.lists.SetListed(request.Context(), currentViewer(request).ID, item.ID, request.Method == http.MethodPost)
	})
	if writeJellyfinOperationError(writer, request, operationErr) {
		return
	}
	jellyfinJSON(writer, api.userDataDTO(request, item))
}

func (api *jellyfinAPI) userDataDTO(request *http.Request, item library.Item) map[string]any {
	state := api.progress.Get(request, item.ID)
	project := sharedjellyfin.ProjectUserDataDTO
	var presentation func(float64) float64
	if viewer := currentViewer(request); viewer.Owner || viewer.Transcode {
		presentation = func(seconds float64) float64 { return api.presentationJellyfinSeconds(request, item, seconds) }
	}
	return project(item, sharedjellyfin.UserState{Seconds: state.Seconds, Watched: state.Watched}, api.lists.Has(request, item.ID), presentation)
}

func (api *jellyfinAPI) presentationJellyfinSeconds(request *http.Request, item library.Item, seconds float64) float64 {
	media := api.probe.inspect(request.Context(), item)
	timeline := automaticSkipTimeline(media.Duration, media.Markers, api.settings.autoSkip())
	if len(timeline.Omitted) > 0 {
		return timeline.PresentationTime(seconds)
	}
	return seconds
}

func (api *jellyfinAPI) saveJellyfinProgress(writer http.ResponseWriter, request *http.Request, id string, ticks int64, watched *bool, session string, revision uint64, stopped ...bool) {
	item, found := visibleItem(request, api.index, jellyfinRawID(id))
	operationErr := sharedjellyfin.SaveProgress(found, ticks, func(seconds float64) (bool, error) {
		return api.progress.SetRevision(request, item.ID, seconds, watched, session, revision)
	}, func(err error) bool { return errors.Is(err, errInvalidProgressState) })
	if writeJellyfinOperationError(writer, request, operationErr) {
		return
	}
	if len(stopped) > 0 && stopped[0] {
		api.plays.Delete(session)
		api.progress.Stop(request, item.ID, float64(ticks)/1e7)
	}
	writer.WriteHeader(http.StatusNoContent)
}

func writeJellyfinOperationError(writer http.ResponseWriter, request *http.Request, err *sharedjellyfin.OperationError) bool {
	if err == nil {
		return false
	}
	if err.Status == http.StatusNotFound {
		http.NotFound(writer, request)
	} else {
		http.Error(writer, err.Message, err.Status)
	}
	return true
}
