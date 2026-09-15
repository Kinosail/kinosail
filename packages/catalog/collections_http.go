package catalog

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"strings"
	"unicode"

	"github.com/MikeO7/kinosail/packages/httpguard"
	"github.com/MikeO7/kinosail/packages/library"
)

// CollectionIndex exposes Viewer-filtered Library reads to the shared API adapter.
type CollectionIndex interface {
	VisibleLibrary(*http.Request) []library.Item
	VisibleItem(*http.Request, string) (library.Item, bool)
}

// CollectionProgress projects Library items into the app's public client shape.
type CollectionProgress interface {
	ClientItems(*http.Request, []library.Item) any
}

// CollectionStore owns Collection queries and durable mutations.
type CollectionStore interface {
	CollectionNames([]library.Item) []string
	CollectionSummaries([]library.Item) any
	Collection(string, []library.Item) []library.Item
	CreateCollection(context.Context, string) error
	SetCollection(context.Context, string, library.Item, bool) error
	DeleteCollection(context.Context, string, []library.Item) error
}

// CollectionHandlers is the shared Player-canonical Collection API.
type CollectionHandlers struct {
	Index    CollectionIndex
	Progress CollectionProgress
	Lists    CollectionStore
}

// List serves the visible Collection index.
func (handlers CollectionHandlers) List(writer http.ResponseWriter, request *http.Request) {
	items := handlers.Index.VisibleLibrary(request)
	writeCollectionJSON(writer, map[string]any{"collections": handlers.Lists.CollectionNames(items), "summaries": handlers.Lists.CollectionSummaries(items)}, http.StatusOK)
}

// Create validates and creates one Collection.
func (handlers CollectionHandlers) Create(writer http.ResponseWriter, request *http.Request) {
	CreateCollectionAPI(request, handlers.Lists.CreateCollection).serve(writer)
}

// Get serves one visible Collection.
func (handlers CollectionHandlers) Get(writer http.ResponseWriter, request *http.Request) {
	items, name := handlers.Index.VisibleLibrary(request), request.PathValue("name")
	CollectionAPI(name, func() []library.Item { return items }, handlers.Lists.CollectionNames, handlers.Lists.Collection, func(values []library.Item) any { return handlers.Progress.ClientItems(request, values) }).serve(writer)
}

// Save validates and updates one Collection item.
func (handlers CollectionHandlers) Save(writer http.ResponseWriter, request *http.Request) {
	SaveCollectionAPI(request, func(id string) (library.Item, bool) { return handlers.Index.VisibleItem(request, id) }, handlers.Lists.SetCollection, collectionStoreStatus).serve(writer)
}

// Delete removes one visible Collection.
func (handlers CollectionHandlers) Delete(writer http.ResponseWriter, request *http.Request) {
	DeleteCollectionAPI(request, func() []library.Item { return handlers.Index.VisibleLibrary(request) }, handlers.Lists.DeleteCollection).serve(writer)
}

// APIAction is one shared JSON mutation or query response.
type APIAction struct {
	Body     any
	Err      error
	Status   int
	NotFound bool
}

func (action APIAction) serve(writer http.ResponseWriter) {
	if action.NotFound {
		action.Err, action.Status = errors.New("not found"), http.StatusNotFound
	}
	if action.Err != nil {
		action.Body = map[string]string{"error": action.Err.Error()}
	}
	if action.Status == http.StatusNoContent {
		writer.WriteHeader(action.Status)
		return
	}
	writeCollectionJSON(writer, action.Body, action.Status)
}

func writeCollectionJSON(writer http.ResponseWriter, value any, status int) {
	writer.Header().Set("Content-Type", "application/json")
	writer.Header().Set("Cache-Control", "no-store")
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(value)
}

func collectionStoreStatus(err error) int {
	if errors.Is(err, os.ErrNotExist) {
		return http.StatusNotFound
	}
	return http.StatusInternalServerError
}

// CreateCollectionAPI strictly parses and creates one Collection.
func CreateCollectionAPI(request *http.Request, create func(context.Context, string) error) APIAction {
	var input struct {
		Name string `json:"name"`
	}
	if err := httpguard.DecodeRequestJSON(nil, request, &input); err != nil {
		return APIAction{Err: err, Status: http.StatusBadRequest}
	}
	input.Name = strings.TrimSpace(input.Name)
	if !ValidListName(input.Name) {
		return APIAction{Err: errors.New("collection name must contain 1 to 64 characters"), Status: http.StatusBadRequest}
	}
	if err := create(request.Context(), input.Name); err != nil {
		return APIAction{Err: err, Status: http.StatusBadRequest}
	}
	return APIAction{Body: map[string]string{"name": input.Name}, Status: http.StatusCreated}
}

// CollectionAPI builds one visible Collection response or a not-found result.
func CollectionAPI(name string, items func() []library.Item, names func([]library.Item) []string, collection func(string, []library.Item) []library.Item, project func([]library.Item) any) APIAction {
	if !validCollectionReadName(name) {
		return APIAction{NotFound: true}
	}
	visible := items()
	if !slicesContains(names(visible), name) {
		return APIAction{NotFound: true}
	}
	return APIAction{Body: map[string]any{"name": name, "items": project(collection(name, visible))}, Status: http.StatusOK}
}

func validCollectionReadName(name string) bool {
	if name == "" || len(name) > 64 || strings.TrimSpace(name) != name || strings.Contains(name, "\\") || strings.ContainsFunc(name, unicode.IsControl) {
		return false
	}
	for _, segment := range strings.Split(name, "/") {
		if segment == "." || segment == ".." {
			return false
		}
	}
	return true
}

// SaveCollectionAPI validates a visible item before mutating Collection membership.
func SaveCollectionAPI(request *http.Request, visible func(string) (library.Item, bool), save func(context.Context, string, library.Item, bool) error, status func(error) int) APIAction {
	var input struct {
		Included bool `json:"included"`
	}
	if err := httpguard.DecodeRequestJSON(nil, request, &input); err != nil {
		return APIAction{Err: err, Status: http.StatusBadRequest}
	}
	name, id := request.PathValue("name"), request.PathValue("id")
	if !ValidListName(name) || !validListItemID(id) {
		return APIAction{Err: errors.New("invalid collection state"), Status: http.StatusBadRequest}
	}
	item, found := visible(id)
	if !found {
		return APIAction{NotFound: true}
	}
	if err := save(request.Context(), name, item, input.Included); err != nil {
		return APIAction{Err: err, Status: status(err)}
	}
	return APIAction{Body: map[string]bool{"included": input.Included}, Status: http.StatusOK}
}

// DeleteCollectionAPI resolves visible items before deleting one Collection.
func DeleteCollectionAPI(request *http.Request, items func() []library.Item, remove func(context.Context, string, []library.Item) error) APIAction {
	name := request.PathValue("name")
	if !ValidListName(name) {
		return APIAction{Err: errors.New("invalid collection state"), Status: http.StatusBadRequest}
	}
	if err := remove(request.Context(), name, items()); err != nil {
		return APIAction{Err: err, Status: http.StatusInternalServerError}
	}
	return APIAction{Status: http.StatusNoContent}
}

func slicesContains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
