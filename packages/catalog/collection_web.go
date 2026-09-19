package catalog

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/MikeO7/kinosail/packages/library"
)

// CollectionWebPage is the app-neutral view model for one Collection page.
type CollectionWebPage struct {
	Name, Query       string
	Items, Candidates []library.Item
	ItemCount         int
	Owner             bool
}

// CollectionWebHandlersConfig binds Collection behavior to app-owned state and views.
type CollectionWebHandlersConfig struct {
	VisibleLibrary  func(*http.Request) ([]library.Item, error)
	Snapshot        func(*http.Request) ([]library.Item, error)
	VisibleItem     func(*http.Request, string) (library.Item, bool)
	CollectionNames func([]library.Item) []string
	Collection      func(string, []library.Item) []library.Item
	Create          func(context.Context, string) error
	Set             func(context.Context, string, library.Item, bool) error
	Delete          func(context.Context, string, []library.Item) error
	Owner           func(*http.Request) bool
	Render          func(http.ResponseWriter, *http.Request, CollectionWebPage) error
	Failure         func(http.ResponseWriter, *http.Request, string, int)
	NotFound        func(http.ResponseWriter, *http.Request)
}

// CollectionWebStore exposes shared Collection state operations to the web adapter.
type CollectionWebStore interface {
	CollectionNames([]library.Item) []string
	Collection(string, []library.Item) []library.Item
	CreateCollection(context.Context, string) error
	SetCollection(context.Context, string, library.Item, bool) error
	DeleteCollection(context.Context, string, []library.Item) error
}

// CollectionWebIndex supplies the request-aware visibility and complete snapshot seams.
type CollectionWebIndex interface {
	VisibleLibrary(*http.Request) []library.Item
	VisibleItem(*http.Request, string) (library.Item, bool)
	Snapshot() ([]library.Item, error)
}

// CollectionWebApp supplies the presentation and localized response seams.
type CollectionWebApp struct {
	Index    CollectionWebIndex
	Owner    func(*http.Request) bool
	Render   func(http.ResponseWriter, *http.Request, CollectionWebPage) error
	Failure  func(http.ResponseWriter, *http.Request, string, int)
	NotFound func(http.ResponseWriter, *http.Request)
}

// CollectionWebHandlers owns shared Collection web behavior and validation.
type CollectionWebHandlers struct{ config CollectionWebHandlersConfig }

// NewCollectionWebHandlers creates one Collection HTTP adapter around app-owned seams.
func NewCollectionWebHandlers(config CollectionWebHandlersConfig) CollectionWebHandlers {
	if !validCollectionWebHandlersConfig(config) {
		panic("invalid Collection HTTP dependencies")
	}
	return CollectionWebHandlers{config: config}
}

// NewCollectionWebHandlersForStore binds common Collection state methods once.
func NewCollectionWebHandlersForStore(store CollectionWebStore, app CollectionWebApp) CollectionWebHandlers {
	return NewCollectionWebHandlers(CollectionWebHandlersConfig{
		VisibleLibrary: func(request *http.Request) ([]library.Item, error) {
			return app.Index.VisibleLibrary(request), nil
		},
		Snapshot: func(*http.Request) ([]library.Item, error) {
			return app.Index.Snapshot()
		},
		VisibleItem:     app.Index.VisibleItem,
		CollectionNames: store.CollectionNames,
		Collection:      store.Collection,
		Create:          store.CreateCollection,
		Set:             store.SetCollection,
		Delete:          store.DeleteCollection,
		Owner:           app.Owner,
		Render:          app.Render,
		Failure:         app.Failure,
		NotFound:        app.NotFound,
	})
}

// ManageItem changes one Collection membership after validating the complete form.
func (handlers CollectionWebHandlers) ManageItem(writer http.ResponseWriter, request *http.Request) {
	if len(request.URL.Query()) != 0 {
		handlers.config.Failure(writer, request, "invalid Collection state", http.StatusBadRequest)
		return
	}
	if err := request.ParseForm(); err != nil || !ValidCurationItemForm(request.PostForm) {
		handlers.config.Failure(writer, request, "invalid Collection state", http.StatusBadRequest)
		return
	}
	name, id := request.PathValue("name"), request.PathValue("id")
	if !ValidCollectionPathName(name) || !ValidCurationItemID(id) {
		handlers.config.Failure(writer, request, "invalid Collection state", http.StatusBadRequest)
		return
	}
	included, _ := strconv.ParseBool(request.PostForm.Get("included"))
	item, found := handlers.config.VisibleItem(request, id)
	if !found {
		handlers.config.NotFound(writer, request)
		return
	}
	if err := handlers.config.Set(request.Context(), name, item, included); errors.Is(err, os.ErrNotExist) {
		handlers.config.NotFound(writer, request)
		return
	} else if err != nil {
		handlers.config.Failure(writer, request, err.Error(), http.StatusInternalServerError)
		return
	}
	target := "/collection/" + url.PathEscape(name)
	if query := strings.TrimSpace(request.PostForm.Get("q")); included && query != "" {
		target += "?q=" + url.QueryEscape(query) + "#collection-search"
	}
	http.Redirect(writer, request, target, http.StatusSeeOther)
}

// Create adds one Collection after applying the shared name boundary.
func (handlers CollectionWebHandlers) Create(writer http.ResponseWriter, request *http.Request) {
	name := strings.TrimSpace(request.FormValue("name"))
	if !ValidListName(name) {
		handlers.config.Failure(writer, request, "collection name must contain 1 to 64 characters", http.StatusBadRequest)
		return
	}
	if err := handlers.config.Create(request.Context(), name); err != nil {
		handlers.config.Failure(writer, request, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(writer, request, "/collection/"+url.PathEscape(name), http.StatusSeeOther)
}

// Save changes one Collection membership from a detail-page form.
func (handlers CollectionWebHandlers) Save(writer http.ResponseWriter, request *http.Request) {
	name, id := request.PathValue("name"), request.PathValue("id")
	if !ValidListName(name) || !ValidCurationItemID(id) {
		handlers.config.Failure(writer, request, "invalid Collection state", http.StatusBadRequest)
		return
	}
	item, found := handlers.config.VisibleItem(request, id)
	included, err := strconv.ParseBool(request.FormValue("included"))
	if !found || err != nil {
		handlers.config.Failure(writer, request, "invalid Collection state", http.StatusBadRequest)
		return
	}
	err = handlers.config.Set(request.Context(), name, item, included)
	if errors.Is(err, os.ErrNotExist) {
		handlers.config.NotFound(writer, request)
		return
	}
	if err != nil {
		handlers.config.Failure(writer, request, err.Error(), http.StatusInternalServerError)
		return
	}
	target := "/watch/"
	if item.Kind == "book" {
		target = "/book/"
	}
	http.Redirect(writer, request, target+item.ID, http.StatusSeeOther)
}

// Delete removes one Collection after resolving the current Library snapshot.
func (handlers CollectionWebHandlers) Delete(writer http.ResponseWriter, request *http.Request) {
	name := request.PathValue("name")
	if !ValidListName(name) {
		handlers.config.Failure(writer, request, "invalid Collection state", http.StatusBadRequest)
		return
	}
	items, _ := handlers.config.Snapshot(request)
	if err := handlers.config.Delete(request.Context(), name, items); err != nil {
		handlers.config.Failure(writer, request, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(writer, request, "/", http.StatusSeeOther)
}

// Browse renders one visible Collection and bounded add-item candidates.
func (handlers CollectionWebHandlers) Browse(writer http.ResponseWriter, request *http.Request) {
	query, err := ParseCurationQuery(request.URL.Query())
	if err != nil {
		handlers.config.Failure(writer, request, "invalid Collection state", http.StatusBadRequest)
		return
	}
	items, _ := handlers.config.VisibleLibrary(request)
	name := request.PathValue("name")
	if !slicesContains(handlers.config.CollectionNames(items), name) {
		handlers.config.NotFound(writer, request)
		return
	}
	members := handlers.config.Collection(name, items)
	writer.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := handlers.config.Render(writer, request, CollectionWebPage{
		Name: name, Query: query, Items: members, Candidates: CurationCandidates(items, members, query), ItemCount: len(members), Owner: handlers.config.Owner(request),
	}); err != nil {
		handlers.config.Failure(writer, request, err.Error(), http.StatusInternalServerError)
	}
}

func validCollectionWebHandlersConfig(config CollectionWebHandlersConfig) bool {
	return config.VisibleLibrary != nil && config.Snapshot != nil && config.VisibleItem != nil && config.CollectionNames != nil && config.Collection != nil && config.Create != nil && config.Set != nil && config.Delete != nil && config.Owner != nil && config.Render != nil && config.Failure != nil && config.NotFound != nil
}
