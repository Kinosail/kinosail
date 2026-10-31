package catalogapi

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/MikeO7/kinosail/packages/httpguard"
	"github.com/MikeO7/kinosail/packages/library"
)

type watchedInput struct {
	Watched *bool `json:"watched"`
	Season  *int  `json:"season"`
}

func (handlers MediaHandlers) markItemWatched(writer http.ResponseWriter, request *http.Request) {
	item, found := handlers.Index.VisibleItem(request, request.PathValue("id"))
	if !found {
		apiAction{notFound: true}.serve(writer)
		return
	}
	var input struct {
		Watched *bool `json:"watched"`
	}
	if err := httpguard.DecodeRequestJSON(nil, request, &input); err != nil || input.Watched == nil {
		apiAction{err: errors.New("watched must be true or false"), status: http.StatusBadRequest}.serve(writer)
		return
	}
	handlers.markWatched(writer, request, "item", []library.Item{item}, *input.Watched)
}

func (handlers MediaHandlers) markShowWatched(writer http.ResponseWriter, request *http.Request) {
	show, found := handlers.Index.VisibleView(request).Show(request.PathValue("id"))
	if !found {
		apiAction{notFound: true}.serve(writer)
		return
	}
	var input watchedInput
	if err := httpguard.DecodeRequestJSON(nil, request, &input); err != nil || input.Watched == nil || input.Season != nil && (*input.Season < 0 || *input.Season > 9999) {
		apiAction{err: errors.New("watched must be true or false, and season a number from 0 to 9999"), status: http.StatusBadRequest}.serve(writer)
		return
	}
	episodes, scope := show.Episodes, "show"
	if input.Season != nil {
		scope, episodes = "season", nil
		for _, episode := range show.Episodes {
			if episode.Season == *input.Season {
				episodes = append(episodes, episode)
			}
		}
	}
	if len(episodes) == 0 {
		apiAction{notFound: true}.serve(writer)
		return
	}
	handlers.markWatched(writer, request, scope, episodes, *input.Watched)
}

func (handlers MediaHandlers) markWatched(writer http.ResponseWriter, request *http.Request, scope string, items []library.Item, watched bool) {
	ids := make([]string, 0, len(items))
	for _, item := range items {
		ids = append(ids, item.ID)
	}
	if err := handlers.Progress.MarkWatched(request, ids, watched); err != nil {
		slog.ErrorContext(request.Context(), "mark watched failed", "operation", "progress.mark-watched", "scope", scope, "items", len(ids), "error", err)
		apiAction{err: err, status: handlers.StoreStatus(err)}.serve(writer)
		return
	}
	slog.InfoContext(request.Context(), "marked watched", "operation", "progress.mark-watched", "scope", scope, "watched", watched, "items", len(ids))
	apiAction{body: map[string]any{"watched": watched, "items": len(ids)}, status: http.StatusOK}.serve(writer)
}

func listedFilter(request *http.Request) (bool, bool) {
	values, present := request.URL.Query()["listed"]
	if !present {
		return false, true
	}
	if len(values) != 1 || values[0] != "true" && values[0] != "false" {
		return false, false
	}
	return values[0] == "true", true
}

func (handlers MediaHandlers) showListed(writer http.ResponseWriter, request *http.Request) {
	show, found := handlers.Index.VisibleView(request).Show(request.PathValue("id"))
	if !found {
		apiAction{notFound: true}.serve(writer)
		return
	}
	if request.Method == http.MethodGet {
		apiAction{body: map[string]bool{"listed": handlers.Lists.Has(request, show.ID)}, status: http.StatusOK}.serve(writer)
		return
	}
	var input struct {
		Listed *bool `json:"listed"`
	}
	if err := httpguard.DecodeRequestJSON(nil, request, &input); err != nil || input.Listed == nil {
		apiAction{err: errors.New("listed must be true or false"), status: http.StatusBadRequest}.serve(writer)
		return
	}
	if err := handlers.Lists.SetListed(request.Context(), handlers.Viewer(request), show.ID, *input.Listed); err != nil {
		apiAction{err: err, status: http.StatusInternalServerError}.serve(writer)
		return
	}
	apiAction{body: map[string]bool{"listed": *input.Listed}, status: http.StatusOK}.serve(writer)
}
