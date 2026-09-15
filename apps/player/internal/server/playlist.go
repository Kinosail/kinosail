package server

import (
	"errors"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/MikeO7/kinosail/packages/library"
	"github.com/MikeO7/kinosail/packages/playerweb"
)

const playlistHTML = playerweb.PlaylistHTML

var playlistView = newLocalizedTemplate("playlist", playlistHTMLWithExport)

type playlistWebItem struct {
	library.Item
	CanEarlier, CanLater bool
}

func browsePlaylist(index *libraryIndex, store *listStore) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		query, err := curationQuery(request)
		if err != nil {
			localizedError(writer, request, "invalid playlist state", http.StatusBadRequest)
			return
		}
		name := request.PathValue("name")
		if !contains(store.PlaylistNames(request), name) {
			localizedNotFound(writer, request)
			return
		}
		items, _ := visibleLibrary(request, index)
		members := store.Playlist(request, name, items)
		webItems := make([]playlistWebItem, len(members))
		for position, item := range members {
			webItems[position] = playlistWebItem{item, position > 0, position+1 < len(members)}
		}
		rule, smart := store.playlistRule(request, name)
		candidates := []library.Item(nil)
		mode, ruleDescription := "Manual", ""
		if !smart {
			candidates = collectionCandidates(items, members, query)
		} else {
			mode, ruleDescription = "Smart", describePlaylistRule(rule)
		}
		writer.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := playlistView.Execute(writer, request, struct {
			Name, Query, Mode, Rule string
			Items                   []playlistWebItem
			Candidates              []library.Item
			ItemCount               int
			Smart                   bool
		}{name, query, mode, ruleDescription, webItems, candidates, len(members), smart}); err != nil {
			localizedError(writer, request, err.Error(), http.StatusInternalServerError)
		}
	}
}

func orderPlaylist(index *libraryIndex, store *listStore) http.HandlerFunc { //nolint:cyclop,gocognit // Validation and the no-side-effect boundary stay adjacent to the shared order commit.
	return func(writer http.ResponseWriter, request *http.Request) {
		if err := request.ParseForm(); err != nil || len(request.PostForm) != 2 || len(request.PostForm["id"]) != 1 || len(request.PostForm["direction"]) != 1 {
			localizedError(writer, request, "invalid playlist order", http.StatusBadRequest)
			return
		}
		name, id, direction := request.PathValue("name"), request.PostForm.Get("id"), request.PostForm.Get("direction")
		if !validListName(name) || !validCurationItemID(id) || !oneOf(direction, "up", "down") {
			localizedError(writer, request, "invalid playlist order", http.StatusBadRequest)
			return
		}
		items, _ := visibleLibrary(request, index)
		members := store.Playlist(request, name, items)
		position := -1
		for index, item := range members {
			if item.ID == id {
				position = index
				break
			}
		}
		target := position - 1
		if direction == "down" {
			target = position + 1
		}
		if position < 0 || target < 0 || target >= len(members) {
			localizedError(writer, request, "invalid playlist order", http.StatusBadRequest)
			return
		}
		members[position], members[target] = members[target], members[position]
		ids := make([]string, len(members))
		for index, item := range members {
			ids[index] = item.ID
		}
		if err := store.Order(request.Context(), currentViewer(request).ID, name, ids); err != nil {
			localizedError(writer, request, err.Error(), http.StatusBadRequest)
			return
		}
		http.Redirect(writer, request, "/playlist/"+url.PathEscape(name), http.StatusSeeOther)
	}
}

func managePlaylistItem(index *libraryIndex, store *listStore) http.HandlerFunc { //nolint:gocognit,cyclop // Strict validation and no-side-effect failures stay in one adapter.
	return func(writer http.ResponseWriter, request *http.Request) {
		if len(request.URL.Query()) != 0 {
			localizedError(writer, request, "invalid playlist state", http.StatusBadRequest)
			return
		}
		if err := request.ParseForm(); err != nil || !validCurationItemForm(request.PostForm) {
			localizedError(writer, request, "invalid playlist state", http.StatusBadRequest)
			return
		}
		name, id := request.PathValue("name"), request.PathValue("id")
		if !validListName(name) || !validCurationItemID(id) {
			localizedError(writer, request, "invalid playlist state", http.StatusBadRequest)
			return
		}
		included, _ := strconv.ParseBool(request.PostForm.Get("included"))
		item, found := visibleItem(request, index, id)
		if !found {
			localizedNotFound(writer, request)
			return
		}
		if err := store.SetPlaylist(request.Context(), currentViewer(request).ID, name, item.ID, included); errors.Is(err, os.ErrNotExist) {
			localizedNotFound(writer, request)
			return
		} else if err != nil {
			localizedError(writer, request, err.Error(), http.StatusInternalServerError)
			return
		}
		target := "/playlist/" + url.PathEscape(name)
		if query := strings.TrimSpace(request.PostForm.Get("q")); included && query != "" {
			target += "?q=" + url.QueryEscape(query) + "#playlist-search"
		}
		http.Redirect(writer, request, target, http.StatusSeeOther)
	}
}

func contains(values []string, value string) bool {
	for _, candidate := range values {
		if candidate == value {
			return true
		}
	}
	return false
}
