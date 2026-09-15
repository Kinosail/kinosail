package catalogapi

import (
	"errors"
	"net/http"
	"strings"

	"github.com/MikeO7/kinosail/packages/catalog"
	"github.com/MikeO7/kinosail/packages/httpguard"
	"github.com/MikeO7/kinosail/packages/library"
	"github.com/MikeO7/kinosail/packages/playback"
)

func (handlers MediaHandlers) createSmartPlaylist(writer http.ResponseWriter, request *http.Request) {
	var input struct {
		Name  string `json:"name"`
		Kind  string `json:"kind"`
		Query string `json:"query"`
		Sort  string `json:"sort"`
	}
	if err := httpguard.DecodeRequestJSON(nil, request, &input); err != nil {
		apiAction{err: err, status: http.StatusBadRequest}.serve(writer)
		return
	}
	err := handlers.Lists.CreateSmart(request.Context(), handlers.Viewer(request), input.Name, catalog.PlaylistRule{Kind: input.Kind, Query: input.Query, Sort: input.Sort})
	if err != nil {
		apiAction{err: err, status: http.StatusBadRequest}.serve(writer)
		return
	}
	apiAction{body: map[string]string{"name": strings.TrimSpace(input.Name)}, status: http.StatusCreated}.serve(writer)
}

func (handlers MediaHandlers) orderPlaylist(writer http.ResponseWriter, request *http.Request) {
	var input struct {
		IDs []string `json:"ids"`
	}
	if err := httpguard.DecodeRequestJSON(nil, request, &input); err != nil {
		apiAction{err: err, status: http.StatusBadRequest}.serve(writer)
		return
	}
	err := handlers.Lists.Order(request.Context(), handlers.Viewer(request), request.PathValue("name"), input.IDs)
	if err != nil {
		apiAction{err: err, status: http.StatusBadRequest}.serve(writer)
		return
	}
	apiAction{body: map[string]any{"ids": input.IDs}, status: http.StatusOK}.serve(writer)
}

func (handlers MediaHandlers) dismissProgress(writer http.ResponseWriter, request *http.Request) {
	item, found := handlers.Index.VisibleItem(request, request.PathValue("id"))
	if !found {
		apiAction{notFound: true}.serve(writer)
		return
	}
	if err := handlers.Progress.Dismiss(request, item.ID); err != nil {
		apiAction{err: err, status: http.StatusInternalServerError}.serve(writer)
		return
	}
	apiAction{status: http.StatusNoContent}.serve(writer)
}

func (handlers MediaHandlers) progress(writer http.ResponseWriter, request *http.Request) {
	var input struct {
		Seconds       float64 `json:"seconds"`
		Watched       *bool   `json:"watched"`
		Session       string  `json:"session"`
		Revision      uint64  `json:"revision"`
		PlaybackToken string  `json:"playbackToken"`
	}
	item, found := handlers.Index.VisibleItem(request, request.PathValue("id"))
	if !found {
		apiAction{notFound: true}.serve(writer)
		return
	}
	if err := httpguard.DecodeRequestJSON(nil, request, &input); err != nil {
		apiAction{err: err, status: http.StatusBadRequest}.serve(writer)
		return
	}
	if input.Seconds < 0 {
		apiAction{err: errors.New("progress seconds cannot be negative"), status: http.StatusBadRequest}.serve(writer)
		return
	}
	timeline, err := handlers.timeline(input.PlaybackToken)
	if err != nil {
		apiAction{err: errors.New("playback token is invalid"), status: http.StatusBadRequest}.serve(writer)
		return
	}
	if len(timeline.Omitted) > 0 {
		input.Seconds = timeline.SourceTime(input.Seconds)
	}
	accepted, err := handlers.Progress.SetRevision(request, item.ID, input.Seconds, input.Watched, input.Session, input.Revision)
	if err != nil {
		apiAction{err: err, status: handlers.StoreStatus(err)}.serve(writer)
		return
	}
	if !accepted {
		apiAction{err: errors.New("progress revision is stale"), status: http.StatusConflict}.serve(writer)
		return
	}
	state := handlers.Progress.Get(request, item.ID)
	if len(timeline.Omitted) > 0 {
		state.Seconds = timeline.PresentationTime(state.Seconds)
	}
	apiAction{body: state, status: http.StatusOK}.serve(writer)
}

func (handlers MediaHandlers) timeline(token string) (timeline playback.Timeline, err error) {
	if token == "" {
		return timeline, nil
	}
	if handlers.Timeline == nil {
		return timeline, errors.New("timeline parser is unavailable")
	}
	return handlers.Timeline(token)
}

func (handlers MediaHandlers) list(writer http.ResponseWriter, request *http.Request) {
	var input struct {
		Listed bool `json:"listed"`
	}
	item, found := handlers.Index.VisibleItem(request, request.PathValue("id"))
	if !found {
		apiAction{notFound: true}.serve(writer)
		return
	}
	if err := httpguard.DecodeRequestJSON(nil, request, &input); err != nil {
		apiAction{err: err, status: http.StatusBadRequest}.serve(writer)
		return
	}
	err := handlers.Lists.SetListed(request.Context(), handlers.Viewer(request), item.ID, input.Listed)
	if err != nil {
		apiAction{err: err, status: http.StatusInternalServerError}.serve(writer)
		return
	}
	apiAction{body: map[string]bool{"listed": input.Listed}, status: http.StatusOK}.serve(writer)
}

func (handlers MediaHandlers) createPlaylist(writer http.ResponseWriter, request *http.Request) {
	var input PlaylistDocument
	if err := httpguard.DecodeRequestJSON(nil, request, &input); err != nil {
		apiAction{err: err, status: http.StatusBadRequest}.serve(writer)
		return
	}
	name, err := CreatePlaylistDocument(request.Context(), handlers.Viewer(request), input, func(id string) (library.Item, bool) {
		return handlers.Index.VisibleItem(request, id)
	}, handlers.Lists.Create)
	if err != nil {
		apiAction{err: err, status: http.StatusBadRequest}.serve(writer)
		return
	}
	apiAction{body: map[string]any{"name": name, "ids": input.IDs}, status: http.StatusCreated}.serve(writer)
}

func (handlers MediaHandlers) savePlaylist(writer http.ResponseWriter, request *http.Request) {
	var input struct {
		Included bool `json:"included"`
	}
	item, found := handlers.Index.VisibleItem(request, request.PathValue("id"))
	if !found {
		apiAction{notFound: true}.serve(writer)
		return
	}
	if err := httpguard.DecodeRequestJSON(nil, request, &input); err != nil {
		apiAction{err: err, status: http.StatusBadRequest}.serve(writer)
		return
	}
	err := handlers.Lists.SetPlaylist(request.Context(), handlers.Viewer(request), request.PathValue("name"), item.ID, input.Included)
	if err != nil {
		apiAction{err: err, status: handlers.StoreStatus(err)}.serve(writer)
		return
	}
	apiAction{body: map[string]bool{"included": input.Included}, status: http.StatusOK}.serve(writer)
}

func (handlers MediaHandlers) deletePlaylist(writer http.ResponseWriter, request *http.Request) {
	err := handlers.Lists.DeletePlaylist(request.Context(), handlers.Viewer(request), request.PathValue("name"))
	if err != nil {
		apiAction{err: err, status: http.StatusInternalServerError}.serve(writer)
		return
	}
	apiAction{status: http.StatusNoContent}.serve(writer)
}
