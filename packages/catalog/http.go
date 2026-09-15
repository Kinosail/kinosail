package catalog

import (
	"context"
	"errors"
	"mime"
	"net/http"
	"net/url"
	"os"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/MikeO7/kinosail/packages/library"
)

const maximumListFormBytes = 1 << 20

type listIndex interface {
	VisibleItem(*http.Request, string) (library.Item, bool)
}

type listMutations interface {
	Create(context.Context, string, string, ...string) error
	CreateSmart(context.Context, string, string, PlaylistRule) error
	SetPlaylist(context.Context, string, string, string, bool) error
	DeletePlaylist(context.Context, string, string) error
	SetListed(context.Context, string, string, bool) error
}

// ListHandlers owns the Player-canonical playlist and My List web mutations.
type ListHandlers struct {
	index    listIndex
	lists    listMutations
	viewer   func(*http.Request) string
	document func(*http.Request, string) (string, error)
	failure  func(http.ResponseWriter, *http.Request, string, int)
	notFound func(http.ResponseWriter, *http.Request)
}

// RescanHandler refreshes an index and redirects successful requests home.
func RescanHandler(index *Index, fail func(http.ResponseWriter, *http.Request, string, int)) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		if err := index.Refresh(request.Context()); err != nil {
			fail(writer, request, err.Error(), http.StatusInternalServerError)
			return
		}
		http.Redirect(writer, request, "/", http.StatusSeeOther)
	}
}

// NewListHandlers binds shared list operations to app-owned Viewer and error adapters.
func NewListHandlers(index listIndex, lists listMutations, viewer func(*http.Request) string, document func(*http.Request, string) (string, error), failure func(http.ResponseWriter, *http.Request, string, int), notFound func(http.ResponseWriter, *http.Request)) ListHandlers {
	return ListHandlers{index: index, lists: lists, viewer: viewer, document: document, failure: failure, notFound: notFound}
}

// CreatePlaylist validates and creates one imported or named playlist.
func (handlers ListHandlers) CreatePlaylist(writer http.ResponseWriter, request *http.Request) {
	CreatePlaylist(request, handlers.viewer(request), func(raw string) (string, error) { return handlers.document(request, raw) }, handlers.lists.Create).Serve(writer, request, handlers.failure, handlers.notFound)
}

// CreateSmart validates and creates one smart playlist.
func (handlers ListHandlers) CreateSmart(writer http.ResponseWriter, request *http.Request) {
	CreateSmartPlaylist(request, handlers.viewer(request), handlers.lists.CreateSmart).Serve(writer, request, handlers.failure, handlers.notFound)
}

// SavePlaylist validates and persists one playlist membership change.
func (handlers ListHandlers) SavePlaylist(writer http.ResponseWriter, request *http.Request) {
	SavePlaylist(request, handlers.viewer(request), func(id string) (library.Item, bool) { return handlers.index.VisibleItem(request, id) }, handlers.lists.SetPlaylist).Serve(writer, request, handlers.failure, handlers.notFound)
}

// DeletePlaylist validates and deletes one playlist.
func (handlers ListHandlers) DeletePlaylist(writer http.ResponseWriter, request *http.Request) {
	DeletePlaylist(request, handlers.viewer(request), handlers.lists.DeletePlaylist).Serve(writer, request, handlers.failure, handlers.notFound)
}

// SaveList validates and persists one My List membership change.
func (handlers ListHandlers) SaveList(writer http.ResponseWriter, request *http.Request) {
	SaveList(request, handlers.viewer(request), func(id string) (library.Item, bool) { return handlers.index.VisibleItem(request, id) }, handlers.lists.SetListed).Serve(writer, request, handlers.failure, handlers.notFound)
}

// WebAction is one transport-neutral mutation response.
type WebAction struct {
	Redirect string
	Err      error
	Status   int
	NotFound bool
}

// Serve writes a shared mutation response through app-owned localization hooks.
func (action WebAction) Serve(writer http.ResponseWriter, request *http.Request, failure func(http.ResponseWriter, *http.Request, string, int), notFound func(http.ResponseWriter, *http.Request)) {
	switch {
	case action.NotFound:
		notFound(writer, request)
	case action.Err != nil:
		failure(writer, request, action.Err.Error(), action.Status)
	default:
		http.Redirect(writer, request, action.Redirect, http.StatusSeeOther)
	}
}

// CreatePlaylist validates one document or named-playlist form before persistence.
func CreatePlaylist(request *http.Request, viewer string, document func(string) (string, error), create func(context.Context, string, string, ...string) error) WebAction { //nolint:cyclop // Document and named creation share one strict boundary.
	values, err := listForm(request, "document", "name")
	if err != nil {
		return badListAction("invalid playlist document")
	}
	raw, rawOK := optionalListValue(values, "document", maximumListFormBytes)
	name, nameOK := optionalListValue(values, "name", 64)
	if !rawOK || !nameOK || raw != "" && name != "" || raw == "" && name == "" {
		return badListAction("invalid playlist document")
	}
	if raw != "" {
		name, err = document(raw)
	} else {
		name = strings.TrimSpace(name)
		if !ValidListName(name) {
			return badListAction("invalid playlist document")
		}
		err = create(request.Context(), viewer, name)
	}
	if err != nil {
		status := http.StatusInternalServerError
		if raw != "" || !ValidListName(name) {
			status = http.StatusBadRequest
		}
		return WebAction{Err: err, Status: status}
	}
	return WebAction{Redirect: "/playlist/" + url.PathEscape(name)}
}

// CreateSmartPlaylist validates a complete smart rule before persistence.
func CreateSmartPlaylist(request *http.Request, viewer string, create func(context.Context, string, string, PlaylistRule) error) WebAction {
	values, err := listForm(request, "name", "kind", "query", "sort")
	if err != nil {
		return badListAction("invalid smart playlist rule")
	}
	name, nameOK := requiredListValue(values, "name", 64)
	kind, kindOK := optionalListValue(values, "kind", 16)
	query, queryOK := optionalListValue(values, "query", 200)
	sortOrder, sortOK := requiredListValue(values, "sort", 16)
	if !nameOK || !kindOK || !queryOK || !sortOK {
		return badListAction("invalid smart playlist rule")
	}
	name = strings.TrimSpace(name)
	if !ValidListName(name) || !oneOf(kind, "", "video", "audio", "book", "photo") || !oneOf(sortOrder, "title", "added", "year") {
		return badListAction("invalid smart playlist rule")
	}
	if err := create(request.Context(), viewer, name, PlaylistRule{Kind: kind, Query: query, Sort: sortOrder}); err != nil {
		return WebAction{Err: err, Status: http.StatusBadRequest}
	}
	return WebAction{Redirect: "/playlist/" + url.PathEscape(name)}
}

// SavePlaylist validates membership before resolving or mutating an item.
func SavePlaylist(request *http.Request, viewer string, visible func(string) (library.Item, bool), save func(context.Context, string, string, string, bool) error) WebAction {
	values, err := listForm(request, "included")
	included, includedOK := listBool(values, "included")
	name, id := request.PathValue("name"), request.PathValue("id")
	if err != nil || !includedOK || !ValidListName(name) || !validListItemID(id) {
		return badListAction("invalid playlist state")
	}
	item, found := visible(id)
	if !found {
		return badListAction("invalid playlist state")
	}
	if err := save(request.Context(), viewer, name, item.ID, included); errors.Is(err, os.ErrNotExist) {
		return WebAction{NotFound: true}
	} else if err != nil {
		return WebAction{Err: err, Status: http.StatusInternalServerError}
	}
	target := "/watch/" + item.ID
	if item.Kind == "book" {
		target = "/book/" + item.ID
	}
	return WebAction{Redirect: target}
}

// DeletePlaylist validates a playlist path before persistence.
func DeletePlaylist(request *http.Request, viewer string, remove func(context.Context, string, string) error) WebAction {
	if _, err := listForm(request); err != nil || !ValidListName(request.PathValue("name")) {
		return badListAction("invalid playlist state")
	}
	if err := remove(request.Context(), viewer, request.PathValue("name")); err != nil {
		return WebAction{Err: err, Status: http.StatusInternalServerError}
	}
	return WebAction{Redirect: "/?view=playlists"}
}

// SaveList validates My List membership before resolving or mutating an item.
func SaveList(request *http.Request, viewer string, visible func(string) (library.Item, bool), save func(context.Context, string, string, bool) error) WebAction {
	values, err := listForm(request, "listed")
	listed, listedOK := listBool(values, "listed")
	id := request.PathValue("id")
	if err != nil || !listedOK || !validListItemID(id) {
		return badListAction("invalid list state")
	}
	item, found := visible(id)
	if !found {
		return badListAction("invalid list state")
	}
	if err := save(request.Context(), viewer, item.ID, listed); err != nil {
		return WebAction{Err: err, Status: http.StatusInternalServerError}
	}
	return WebAction{Redirect: "/watch/" + item.ID}
}

// ListedItems selects every item accepted by the app-owned Viewer predicate.
func ListedItems(items []library.Item, has func(string) bool) []library.Item {
	listed := make([]library.Item, 0)
	for _, item := range items {
		if has(item.ID) {
			listed = append(listed, item)
		}
	}
	return listed
}

func listForm(request *http.Request, keys ...string) (url.Values, error) {
	mediaType, _, err := mime.ParseMediaType(request.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/x-www-form-urlencoded" || request.URL.RawQuery != "" {
		return nil, errors.New("invalid form")
	}
	request.Body = http.MaxBytesReader(nil, request.Body, maximumListFormBytes)
	if err := request.ParseForm(); err != nil {
		return nil, err
	}
	for key := range request.PostForm {
		if !slices.Contains(keys, key) {
			return nil, errors.New("invalid form")
		}
	}
	return request.PostForm, nil
}

func requiredListValue(values url.Values, key string, limit int) (string, bool) {
	value, ok := optionalListValue(values, key, limit)
	return value, ok && value != ""
}

func optionalListValue(values url.Values, key string, limit int) (string, bool) {
	selected, found := values[key]
	if !found {
		return "", true
	}
	returnValue := ""
	if len(selected) == 1 {
		returnValue = selected[0]
	}
	return returnValue, len(selected) == 1 && len(returnValue) <= limit && utf8.ValidString(returnValue)
}

func listBool(values url.Values, key string) (bool, bool) {
	value, ok := requiredListValue(values, key, 5)
	result, err := strconv.ParseBool(value)
	return result, ok && err == nil
}

func validListItemID(id string) bool { return id != "" && len(id) <= 512 }

func badListAction(message string) WebAction {
	return WebAction{Err: errors.New(message), Status: http.StatusBadRequest}
}
