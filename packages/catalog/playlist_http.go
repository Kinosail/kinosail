package catalog

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"os"
	"slices"
	"strconv"
	"strings"

	"github.com/MikeO7/kinosail/packages/library"
)

// PlaylistPage is the app-neutral view model for a manual or smart playlist.
type PlaylistPage struct {
	Name, Query, Mode, Rule string
	Items                   []PlaylistItem
	Candidates              []library.Item
	ItemCount               int
	Smart                   bool
}

// PlaylistItem contains the navigation state for one visible playlist item.
type PlaylistItem struct {
	library.Item
	CanEarlier, CanLater bool
}

// PlaylistHandlersConfig binds shared playlist behavior to app-owned state and views.
type PlaylistHandlersConfig struct {
	VisibleLibrary func(*http.Request) ([]library.Item, error)
	PlaylistNames  func(*http.Request) []string
	Playlist       func(*http.Request, string, []library.Item) []library.Item
	PlaylistRule   func(*http.Request, string) (PlaylistRule, bool)
	VisibleItem    func(*http.Request, string) (library.Item, bool)
	Viewer         func(*http.Request) string
	SetPlaylist    func(context.Context, string, string, string, bool) error
	Order          func(context.Context, string, string, []string) error
	Render         func(http.ResponseWriter, *http.Request, PlaylistPage) error
	Failure        func(http.ResponseWriter, *http.Request, string, int)
	NotFound       func(http.ResponseWriter, *http.Request)
}

// PlaylistHandlers owns the shared web behavior for playlist browsing and mutations.
type PlaylistHandlers struct{ config PlaylistHandlersConfig }

// NewPlaylistHandlers creates one playlist HTTP adapter around app-owned seams.
func NewPlaylistHandlers(config PlaylistHandlersConfig) PlaylistHandlers {
	if !validPlaylistHandlersConfig(config) {
		panic("invalid playlist HTTP dependencies")
	}
	return PlaylistHandlers{config: config}
}

// Browse renders one visible playlist and its bounded add-item candidates.
func (handlers PlaylistHandlers) Browse(writer http.ResponseWriter, request *http.Request) {
	query, err := ParseCurationQuery(request.URL.Query())
	if err != nil {
		handlers.config.Failure(writer, request, "invalid playlist state", http.StatusBadRequest)
		return
	}
	name := request.PathValue("name")
	if !slices.Contains(handlers.config.PlaylistNames(request), name) {
		handlers.config.NotFound(writer, request)
		return
	}
	items, _ := handlers.config.VisibleLibrary(request)
	members := handlers.config.Playlist(request, name, items)
	pageItems := make([]PlaylistItem, len(members))
	for position, item := range members {
		pageItems[position] = PlaylistItem{Item: item, CanEarlier: position > 0, CanLater: position+1 < len(members)}
	}
	rule, smart := handlers.config.PlaylistRule(request, name)
	mode, description := "Manual", ""
	candidates := []library.Item(nil)
	if smart {
		mode, description = "Smart", DescribePlaylistRule(rule)
	} else {
		candidates = PlaylistCandidates(items, members, query)
	}
	writer.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := handlers.config.Render(writer, request, PlaylistPage{
		Name: name, Query: query, Mode: mode, Rule: description, Items: pageItems,
		Candidates: candidates, ItemCount: len(members), Smart: smart,
	}); err != nil {
		handlers.config.Failure(writer, request, err.Error(), http.StatusInternalServerError)
	}
}

// Order changes one playlist item's position after validating the complete form.
func (handlers PlaylistHandlers) Order(writer http.ResponseWriter, request *http.Request) {
	if err := request.ParseForm(); err != nil || len(request.PostForm) != 2 || len(request.PostForm["id"]) != 1 || len(request.PostForm["direction"]) != 1 {
		handlers.config.Failure(writer, request, "invalid playlist order", http.StatusBadRequest)
		return
	}
	name, id, direction := request.PathValue("name"), request.PostForm.Get("id"), request.PostForm.Get("direction")
	if !ValidListName(name) || !ValidCurationItemID(id) || !oneOf(direction, "up", "down") {
		handlers.config.Failure(writer, request, "invalid playlist order", http.StatusBadRequest)
		return
	}
	items, _ := handlers.config.VisibleLibrary(request)
	members := handlers.config.Playlist(request, name, items)
	position := slices.IndexFunc(members, func(item library.Item) bool { return item.ID == id })
	target := position - 1
	if direction == "down" {
		target = position + 1
	}
	if position < 0 || target < 0 || target >= len(members) {
		handlers.config.Failure(writer, request, "invalid playlist order", http.StatusBadRequest)
		return
	}
	members[position], members[target] = members[target], members[position]
	ids := make([]string, len(members))
	for position, item := range members {
		ids[position] = item.ID
	}
	if err := handlers.config.Order(request.Context(), handlers.config.Viewer(request), name, ids); err != nil {
		handlers.config.Failure(writer, request, err.Error(), http.StatusBadRequest)
		return
	}
	http.Redirect(writer, request, "/playlist/"+url.PathEscape(name), http.StatusSeeOther)
}

// ManageItem changes one playlist membership after validating the complete form.
func (handlers PlaylistHandlers) ManageItem(writer http.ResponseWriter, request *http.Request) {
	if len(request.URL.Query()) != 0 {
		handlers.config.Failure(writer, request, "invalid playlist state", http.StatusBadRequest)
		return
	}
	if err := request.ParseForm(); err != nil || !ValidCurationItemForm(request.PostForm) {
		handlers.config.Failure(writer, request, "invalid playlist state", http.StatusBadRequest)
		return
	}
	name, id := request.PathValue("name"), request.PathValue("id")
	if !ValidListName(name) || !ValidCurationItemID(id) {
		handlers.config.Failure(writer, request, "invalid playlist state", http.StatusBadRequest)
		return
	}
	included, _ := strconv.ParseBool(request.PostForm.Get("included"))
	item, found := handlers.config.VisibleItem(request, id)
	if !found {
		handlers.config.NotFound(writer, request)
		return
	}
	if err := handlers.config.SetPlaylist(request.Context(), handlers.config.Viewer(request), name, item.ID, included); errors.Is(err, os.ErrNotExist) {
		handlers.config.NotFound(writer, request)
		return
	} else if err != nil {
		handlers.config.Failure(writer, request, err.Error(), http.StatusInternalServerError)
		return
	}
	target := "/playlist/" + url.PathEscape(name)
	if query := strings.TrimSpace(request.PostForm.Get("q")); included && query != "" {
		target += "?q=" + url.QueryEscape(query) + "#playlist-search"
	}
	http.Redirect(writer, request, target, http.StatusSeeOther)
}

// ParseCurationQuery accepts only the shared Collection and playlist browse query.
func ParseCurationQuery(values url.Values) (string, error) {
	for key := range values {
		if key != "q" && key != "lang" {
			return "", ErrInvalidBrowse
		}
	}
	return SingleValue(values, "q", 200)
}

// PlaylistCandidates returns bounded visible items not already in a manual playlist.
func PlaylistCandidates(items, members []library.Item, query string) []library.Item {
	if query == "" {
		return nil
	}
	included := make(map[string]bool, len(members))
	for _, item := range members {
		included[item.ID] = true
	}
	candidates := make([]library.Item, 0)
	for _, item := range Filter(items, query) {
		if !included[item.ID] {
			candidates = append(candidates, item)
		}
	}
	candidates = Sort(candidates, "title")
	return candidates[:min(48, len(candidates))]
}

func validPlaylistHandlersConfig(config PlaylistHandlersConfig) bool {
	return config.VisibleLibrary != nil && config.PlaylistNames != nil && config.Playlist != nil && config.PlaylistRule != nil && config.VisibleItem != nil && config.Viewer != nil && config.SetPlaylist != nil && config.Order != nil && config.Render != nil && config.Failure != nil && config.NotFound != nil
}
