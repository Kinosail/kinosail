package catalog

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/MikeO7/kinosail/packages/documentdb"
	"github.com/MikeO7/kinosail/packages/library"
)

func TestListMutationErrorEdges(t *testing.T) { //nolint:cyclop,funlen // One table proves each mutation failure is translated without a later effect.
	t.Parallel()
	wantErr := errors.New("failed")
	failed := 0
	(WebAction{Err: wantErr, Status: http.StatusConflict}).Serve(httptest.NewRecorder(), listRequest("/", ""), func(http.ResponseWriter, *http.Request, string, int) { failed++ }, func(http.ResponseWriter, *http.Request) {})
	if failed != 1 {
		t.Fatal("web action did not use failure adapter")
	}
	if action := CreatePlaylist(listRequest("/", "name=Queue"), "viewer", func(string) (string, error) { return "", nil }, func(context.Context, string, string, ...string) error { return wantErr }); action.Status != http.StatusInternalServerError {
		t.Fatalf("named create error = %#v", action)
	}
	for _, document := range []func(string) (string, error){
		func(string) (string, error) { return "Imported", wantErr },
		func(string) (string, error) { return "bad/name", wantErr },
	} {
		if action := CreatePlaylist(listRequest("/", "document=data"), "viewer", document, func(context.Context, string, string, ...string) error { t.Fatal("unexpected create"); return nil }); action.Status != http.StatusBadRequest {
			t.Fatalf("document error = %#v", action)
		}
	}
	if action := CreateSmartPlaylist(listRequest("/", "name=Queue&sort=title"), "viewer", func(context.Context, string, string, PlaylistRule) error { return wantErr }); action.Status != http.StatusBadRequest {
		t.Fatalf("smart create error = %#v", action)
	}
	invalidSmart := listRequest("/", "name=Queue&sort=title")
	invalidSmart.Header.Set("Content-Type", "text/plain")
	if action := CreateSmartPlaylist(invalidSmart, "viewer", func(context.Context, string, string, PlaylistRule) error { t.Fatal("unexpected create"); return nil }); action.Status != http.StatusBadRequest {
		t.Fatalf("invalid smart form = %#v", action)
	}
	visible := func(string) (library.Item, bool) { return library.Item{ID: "item", Kind: "video"}, true }
	playlistRequest := listRequest("/", "included=true")
	playlistRequest.SetPathValue("name", "Queue")
	playlistRequest.SetPathValue("id", "item")
	if action := SavePlaylist(playlistRequest, "viewer", func(string) (library.Item, bool) { return library.Item{}, false }, func(context.Context, string, string, string, bool) error { t.Fatal("unexpected save"); return nil }); action.Status != http.StatusBadRequest {
		t.Fatalf("invisible playlist item = %#v", action)
	}
	if action := SavePlaylist(playlistRequest, "viewer", visible, func(context.Context, string, string, string, bool) error { return wantErr }); action.Status != http.StatusInternalServerError {
		t.Fatalf("playlist save error = %#v", action)
	}
	if action := SavePlaylist(playlistRequest, "viewer", visible, func(context.Context, string, string, string, bool) error { return nil }); action.Redirect != "/watch/item" {
		t.Fatalf("video playlist redirect = %#v", action)
	}
	deleteRequest := listRequest("/", "")
	deleteRequest.SetPathValue("name", "Queue")
	if action := DeletePlaylist(deleteRequest, "viewer", func(context.Context, string, string) error { return wantErr }); action.Status != http.StatusInternalServerError {
		t.Fatalf("playlist delete error = %#v", action)
	}
	listRequestValue := listRequest("/", "listed=true")
	listRequestValue.SetPathValue("id", "item")
	if action := SaveList(listRequestValue, "viewer", visible, func(context.Context, string, string, bool) error { return wantErr }); action.Status != http.StatusInternalServerError {
		t.Fatalf("list save error = %#v", action)
	}
	if action := SaveList(listRequest("/", ""), "viewer", visible, func(context.Context, string, string, bool) error { t.Fatal("unexpected save"); return nil }); action.Status != http.StatusBadRequest {
		t.Fatalf("invalid list request = %#v", action)
	}
	invalid := listRequest("/", strings.Repeat("x", maximumListFormBytes+1))
	if _, err := listForm(invalid); err == nil {
		t.Fatal("oversized list form was accepted")
	}
}

func TestListHandlerDocumentAndCollectionFailures(t *testing.T) {
	t.Parallel()
	mutations := &listMutationStub{}
	documents := 0
	handlers := NewListHandlers(&listIndexStub{}, mutations, func(*http.Request) string { return "viewer" }, func(_ *http.Request, raw string) (string, error) { documents++; return raw, nil }, func(writer http.ResponseWriter, _ *http.Request, _ string, status int) { writer.WriteHeader(status) }, func(http.ResponseWriter, *http.Request) {})
	written := httptest.NewRecorder()
	handlers.CreatePlaylist(written, listRequest("/", "document=Imported"))
	if written.Code != http.StatusSeeOther || documents != 1 {
		t.Fatalf("document handler = %d, calls %d", written.Code, documents)
	}
	wantErr := errors.New("failed")
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/collections", strings.NewReader(`{"name":"Queue"}`))
	request.Header.Set("Content-Type", "application/json")
	if action := CreateCollectionAPI(request, func(context.Context, string) error { return wantErr }); action.Status != http.StatusBadRequest || !errors.Is(action.Err, wantErr) {
		t.Fatalf("collection create failure = %#v", action)
	}
}

func TestListStorageAndProgressRemainingEdges(t *testing.T) { //nolint:cyclop // Persistence and progress failures share the same no-publication invariant.
	t.Parallel()
	values := map[string]bool{}
	playlists := map[string]map[string]bool{}
	order := map[string][]string{}
	smart := map[string]PlaylistRule{}
	loadErr := error(nil)
	wantErr := errors.New("failed")
	storage := ListStorage{Values: &values, Playlists: &playlists, PlaylistOrder: &order, Smart: &smart, Paths: ListPaths{Values: "lists.json"}, Persist: func(string, any) error { return wantErr }, LoadError: &loadErr}
	if err := storage.Commit(t.Context(), CloneListState(nil, nil, nil, nil), ListedDocument); !errors.Is(err, wantErr) {
		t.Fatalf("list commit error = %v", err)
	}
	if _, err := LoadListState(ListPaths{}, func(string, any) (bool, error) { return false, wantErr }); !errors.Is(err, wantErr) {
		t.Fatalf("list load error = %v", err)
	}
	if _, err := LoadListState(ListPaths{}, func(_ string, target any) (bool, error) {
		if values, ok := target.(*map[string]bool); ok {
			*values = map[string]bool{"": true}
		}
		return true, nil
	}); err == nil {
		t.Fatal("invalid loaded list state was accepted")
	}
	state := CloneListState(nil, nil, nil, nil)
	if state.Values == nil || state.Playlists == nil || state.PlaylistOrder == nil || state.Smart == nil {
		t.Fatalf("uninitialized state = %#v", state)
	}
	database, err := documentdb.Open(t.TempDir(), true, documentdb.PlayerConfig())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })
	if err := CommitListState(t.Context(), database, ListPaths{}, nil, state, uint8(ListedDocument)); err != nil {
		t.Fatalf("database list commit = %v", err)
	}
	if err := CommitListState(t.Context(), nil, ListPaths{Values: "lists.json"}, func(string, any) error { return wantErr }, state, uint8(ListedDocument)); !errors.Is(err, wantErr) {
		t.Fatalf("file list commit = %v", err)
	}
	progress := map[string]PlaybackState{}
	var mutex, persistMutex sync.Mutex
	progressStorage := ProgressStorage{Mutex: &mutex, PersistMutex: &persistMutex, Values: &progress}
	if _, _, changed, err := progressStorage.Update("item", func(current PlaybackState) (PlaybackState, bool, error) { return current, false, nil }); err != nil || changed {
		t.Fatalf("unchanged progress = %t, %v", changed, err)
	}
	if ValidateStoredProgress(nil) != nil || ValidateStoredProgress(map[string]PlaybackState{"": {}}) == nil {
		t.Fatal("stored progress validation missed an edge")
	}
}

func TestStoredProgressRejectsExcessiveCardinality(t *testing.T) {
	values := make(map[string]PlaybackState, 1_000_001)
	for index := 0; index <= 1_000_000; index++ {
		values[strconv.Itoa(index)] = PlaybackState{}
	}
	if ValidateStoredProgress(values) == nil {
		t.Fatal("excessive progress cardinality was accepted")
	}
}
