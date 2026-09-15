package catalog

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail/packages/library"
)

func TestCollectionHandlersValidateBeforeMutation(t *testing.T) { //nolint:cyclop,gocognit,funlen // One matrix proves the complete shared API boundary.
	index := &collectionIndexStub{items: []library.Item{{ID: "one"}}, visible: library.Item{ID: "one"}, found: true}
	progress := &collectionProgressStub{}
	store := &collectionStoreStub{names: []string{"Favorites"}, selected: []library.Item{{ID: "one"}}}
	handlers := CollectionHandlers{Index: index, Progress: progress, Lists: store}

	assertResponse := func(method, target, body string, handler http.HandlerFunc, status int) {
		t.Helper()
		request := httptest.NewRequestWithContext(t.Context(), method, target, strings.NewReader(body))
		request.Header.Set("Content-Type", "application/json")
		parts := strings.Split(strings.Trim(target, "/"), "/")
		for position, part := range parts {
			if part == "collections" && position+1 < len(parts) {
				name, _ := url.PathUnescape(parts[position+1])
				request.SetPathValue("name", name)
			}
			if part == "items" && position+1 < len(parts) {
				id, _ := url.PathUnescape(parts[position+1])
				request.SetPathValue("id", id)
			}
		}
		writer := httptest.NewRecorder()
		handler(writer, request)
		if writer.Code != status || writer.Header().Get("Cache-Control") != "no-store" && status != http.StatusNoContent {
			t.Fatalf("%s %s = %d, headers=%v, body=%q", method, target, writer.Code, writer.Header(), writer.Body.String())
		}
	}

	assertResponse(http.MethodGet, "/collections", "", handlers.List, http.StatusOK)
	assertResponse(http.MethodPost, "/collections", `{"name":" Favorites "}`, handlers.Create, http.StatusCreated)
	assertResponse(http.MethodGet, "/collections/Favorites", "", handlers.Get, http.StatusOK)
	assertResponse(http.MethodPut, "/collections/Favorites/items/one", `{"included":true}`, handlers.Save, http.StatusOK)
	assertResponse(http.MethodDelete, "/collections/Favorites", "", handlers.Delete, http.StatusNoContent)
	if store.creates != 1 || store.saves != 1 || store.deletes != 1 || store.lastName != "Favorites" || !store.lastIncluded || progress.calls != 1 {
		t.Fatalf("mutations=%d/%d/%d name=%q included=%t projections=%d", store.creates, store.saves, store.deletes, store.lastName, store.lastIncluded, progress.calls)
	}

	for _, invalid := range []string{`{`, `{"unknown":true}`, `{"name":""}`, `{"name":"bad/name"}`, `{"name":"` + strings.Repeat("x", 65) + `"}`} {
		assertResponse(http.MethodPost, "/collections", invalid, handlers.Create, http.StatusBadRequest)
	}
	if store.creates != 1 {
		t.Fatalf("invalid create caused %d mutations", store.creates)
	}

	assertResponse(http.MethodGet, "/collections/bad%2Fname", "", handlers.Get, http.StatusNotFound)
	assertResponse(http.MethodGet, "/collections/Missing", "", handlers.Get, http.StatusNotFound)
	assertResponse(http.MethodPut, "/collections/Favorites/items/one", `{`, handlers.Save, http.StatusBadRequest)
	assertResponse(http.MethodPut, "/collections/bad%2Fname/items/one", `{"included":true}`, handlers.Save, http.StatusBadRequest)
	assertResponse(http.MethodPut, "/collections/Favorites/items/"+strings.Repeat("x", 513), `{"included":true}`, handlers.Save, http.StatusBadRequest)
	index.found = false
	assertResponse(http.MethodPut, "/collections/Favorites/items/missing", `{"included":true}`, handlers.Save, http.StatusNotFound)
	index.found = true
	store.saveErr = os.ErrNotExist
	assertResponse(http.MethodPut, "/collections/Favorites/items/one", `{"included":true}`, handlers.Save, http.StatusNotFound)
	store.saveErr = errors.New("disk")
	assertResponse(http.MethodPut, "/collections/Favorites/items/one", `{"included":true}`, handlers.Save, http.StatusInternalServerError)
	if store.saves != 3 || index.itemLookups != 4 {
		t.Fatalf("invalid save crossed boundary: saves=%d lookups=%d", store.saves, index.itemLookups)
	}

	reads, deletes := index.reads, store.deletes
	assertResponse(http.MethodDelete, "/collections/bad%2Fname", "", handlers.Delete, http.StatusBadRequest)
	if index.reads != reads || store.deletes != deletes {
		t.Fatal("invalid delete read or mutated collection state")
	}
	store.error = errors.New("disk")
	assertResponse(http.MethodDelete, "/collections/Favorites", "", handlers.Delete, http.StatusInternalServerError)
}

func TestListActionsValidateBeforePersistence(t *testing.T) { //nolint:cyclop,gocognit,funlen // One matrix covers each web mutation and its rejection boundary.
	calls := 0
	create := func(context.Context, string, string, ...string) error { calls++; return nil }
	document := func(raw string) (string, error) { calls++; return raw, nil }
	request := listRequest("/playlists", "name=Queue")
	if action := CreatePlaylist(request, "viewer", document, create); action.Err != nil || action.Redirect != "/playlist/Queue" || calls != 1 {
		t.Fatalf("create = %#v calls=%d", action, calls)
	}
	request = listRequest("/playlists", "document=Imported")
	if action := CreatePlaylist(request, "viewer", document, create); action.Err != nil || action.Redirect != "/playlist/Imported" || calls != 2 {
		t.Fatalf("import = %#v calls=%d", action, calls)
	}
	for _, body := range []string{"", "name=A&document=B", "unknown=x", "name=A&name=B", "name=%FF", "name=bad%2Fname", "name=" + strings.Repeat("x", 65)} {
		if action := CreatePlaylist(listRequest("/playlists", body), "viewer", document, create); action.Status != http.StatusBadRequest {
			t.Fatalf("invalid create %q = %#v", body, action)
		}
	}
	wrong := listRequest("/playlists", "name=Queue")
	wrong.Header.Set("Content-Type", "text/plain")
	if action := CreatePlaylist(wrong, "viewer", document, create); action.Status != http.StatusBadRequest || calls != 2 {
		t.Fatalf("wrong content type = %#v calls=%d", action, calls)
	}

	smartCalls := 0
	smart := func(_ context.Context, viewer, name string, rule PlaylistRule) error {
		smartCalls++
		if viewer != "viewer" || name != "New" || rule.Sort != "title" {
			t.Fatalf("smart input = %q %q %#v", viewer, name, rule)
		}
		return nil
	}
	if action := CreateSmartPlaylist(listRequest("/smart", "name=New&kind=video&query=test&sort=title"), "viewer", smart); action.Err != nil || smartCalls != 1 {
		t.Fatalf("smart = %#v calls=%d", action, smartCalls)
	}
	if action := CreateSmartPlaylist(listRequest("/smart", "name=New&sort="), "viewer", smart); action.Status != http.StatusBadRequest || smartCalls != 1 {
		t.Fatalf("invalid smart = %#v calls=%d", action, smartCalls)
	}
	for _, body := range []string{"name=%FF&sort=title", "name=bad%2Fname&sort=title", "name=New&kind=unknown&sort=title", "name=New&sort=unknown"} {
		if action := CreateSmartPlaylist(listRequest("/smart", body), "viewer", smart); action.Status != http.StatusBadRequest || smartCalls != 1 {
			t.Fatalf("invalid smart %q = %#v calls=%d", body, action, smartCalls)
		}
	}

	visibleCalls, saveCalls := 0, 0
	visible := func(id string) (library.Item, bool) { visibleCalls++; return library.Item{ID: id, Kind: "book"}, true }
	save := func(context.Context, string, string, string, bool) error { saveCalls++; return nil }
	request = listRequest("/playlist/Queue/item", "included=true")
	request.SetPathValue("name", "Queue")
	request.SetPathValue("id", "item")
	if action := SavePlaylist(request, "viewer", visible, save); action.Err != nil || action.Redirect != "/book/item" || visibleCalls != 1 || saveCalls != 1 {
		t.Fatalf("save playlist = %#v visible=%d saves=%d", action, visibleCalls, saveCalls)
	}
	request = listRequest("/playlist/Queue/item", "included=maybe")
	request.SetPathValue("name", "Queue")
	request.SetPathValue("id", "item")
	if action := SavePlaylist(request, "viewer", visible, save); action.Status != http.StatusBadRequest || visibleCalls != 1 || saveCalls != 1 {
		t.Fatalf("invalid playlist save = %#v", action)
	}

	listSave := func(context.Context, string, string, bool) error { saveCalls++; return nil }
	request = listRequest("/list/item", "listed=false")
	request.SetPathValue("id", "item")
	if action := SaveList(request, "viewer", visible, listSave); action.Err != nil || action.Redirect != "/watch/item" {
		t.Fatalf("save list = %#v", action)
	}
	request = listRequest("/list/item", "listed=true")
	request.SetPathValue("id", "item")
	if action := SaveList(request, "viewer", func(string) (library.Item, bool) { return library.Item{}, false }, listSave); action.Status != http.StatusBadRequest {
		t.Fatalf("missing item = %#v", action)
	}

	deleteCalls := 0
	remove := func(context.Context, string, string) error { deleteCalls++; return nil }
	request = listRequest("/playlist/Queue/delete", "")
	request.SetPathValue("name", "Queue")
	if action := DeletePlaylist(request, "viewer", remove); action.Err != nil || action.Redirect != "/?view=playlists" || deleteCalls != 1 {
		t.Fatalf("delete = %#v calls=%d", action, deleteCalls)
	}
	request = listRequest("/playlist/bad/delete", "unknown=true")
	request.SetPathValue("name", "bad/name")
	if action := DeletePlaylist(request, "viewer", remove); action.Status != http.StatusBadRequest || deleteCalls != 1 {
		t.Fatalf("invalid delete = %#v calls=%d", action, deleteCalls)
	}
	if listed := ListedItems([]library.Item{{ID: "one"}, {ID: "two"}}, func(id string) bool { return id == "two" }); len(listed) != 1 || listed[0].ID != "two" {
		t.Fatalf("listed = %#v", listed)
	}
}

func TestSharedListHandlersAndResponses(t *testing.T) {
	index, mutations := &listIndexStub{item: library.Item{ID: "one"}, found: true}, &listMutationStub{}
	failures, missing := 0, 0
	handlers := NewListHandlers(index, mutations, func(*http.Request) string { return "viewer" }, func(_ *http.Request, raw string) (string, error) { return raw, nil }, func(writer http.ResponseWriter, _ *http.Request, _ string, status int) {
		failures++
		writer.WriteHeader(status)
	}, func(writer http.ResponseWriter, _ *http.Request) { missing++; writer.WriteHeader(http.StatusNotFound) })
	cases := []struct {
		target, body string
		handler      http.HandlerFunc
	}{
		{"/playlists", "name=Queue", handlers.CreatePlaylist},
		{"/smart", "name=New&sort=title", handlers.CreateSmart},
		{"/playlist/Queue/one", "included=true", handlers.SavePlaylist},
		{"/playlist/Queue/delete", "", handlers.DeletePlaylist},
		{"/list/one", "listed=true", handlers.SaveList},
	}
	for _, test := range cases {
		request := listRequest(test.target, test.body)
		request.SetPathValue("name", "Queue")
		request.SetPathValue("id", "one")
		writer := httptest.NewRecorder()
		test.handler(writer, request)
		if writer.Code != http.StatusSeeOther {
			t.Fatalf("%s = %d, body=%q", test.target, writer.Code, writer.Body.String())
		}
	}
	mutations.err = os.ErrNotExist
	request := listRequest("/playlist/Queue/one", "included=true")
	request.SetPathValue("name", "Queue")
	request.SetPathValue("id", "one")
	handlers.SavePlaylist(httptest.NewRecorder(), request)
	if mutations.calls != 6 || failures != 0 || missing != 1 {
		t.Fatalf("calls=%d failures=%d missing=%d", mutations.calls, failures, missing)
	}
}
