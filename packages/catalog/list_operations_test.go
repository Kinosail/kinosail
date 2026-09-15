package catalog

import (
	"context"
	"errors"
	"reflect"
	"slices"
	"strings"
	"testing"
)

func listOperationStorage(persist func(string, any) error) ListStorage {
	state := CloneListState(nil, map[string]map[string]bool{"viewer:Queue": {"a": true, "b": true}}, map[string][]string{"viewer:Queue": {"a", "b"}}, map[string]PlaylistRule{"viewer:Smart": {Sort: "title"}})
	var loadErr error
	return ListStorage{Values: &state.Values, Playlists: &state.Playlists, PlaylistOrder: &state.PlaylistOrder, Smart: &state.Smart, Paths: ListPaths{"values", "playlists", "order", "smart"}, Persist: persist, LoadError: &loadErr}
}

func TestListOperationsKeepMemoryOnPersistenceFailure(t *testing.T) {
	t.Parallel()
	failure := errors.New("storage blocked")
	for name, operation := range map[string]func(ListStorage) error{
		"create": func(s ListStorage) error { return s.Create(t.Context(), "viewer", "New", "a") },
		"smart": func(s ListStorage) error {
			return s.CreateSmart(t.Context(), "viewer", "New", PlaylistRule{Sort: "added"})
		},
		"include": func(s ListStorage) error { return s.SetPlaylist(t.Context(), "viewer", "Queue", "c", true) },
		"remove":  func(s ListStorage) error { return s.SetPlaylist(t.Context(), "viewer", "Queue", "a", false) },
		"order":   func(s ListStorage) error { return s.Order(t.Context(), "viewer", "Queue", []string{"b", "a"}) },
		"delete":  func(s ListStorage) error { return s.DeletePlaylist(t.Context(), "viewer", "Queue") },
		"listed":  func(s ListStorage) error { return s.SetListed(t.Context(), "viewer", "c", true) },
	} {
		t.Run(name, func(t *testing.T) {
			storage := listOperationStorage(func(string, any) error { return failure })
			before := storage.State()
			if err := operation(storage); !errors.Is(err, failure) {
				t.Fatalf("error=%v", err)
			}
			if !reflect.DeepEqual(before, storage.State()) {
				t.Fatal("failed persistence changed memory")
			}
		})
	}
}

func TestListOperationsRejectBeforePersistence(t *testing.T) {
	t.Parallel()
	for name, operation := range map[string]func(ListStorage) error{
		"empty name":      func(s ListStorage) error { return s.Create(t.Context(), "viewer", " ") },
		"long name":       func(s ListStorage) error { return s.Create(t.Context(), "viewer", strings.Repeat("n", 65)) },
		"path name":       func(s ListStorage) error { return s.Create(t.Context(), "viewer", "a/b") },
		"too many IDs":    func(s ListStorage) error { return s.Create(t.Context(), "viewer", "New", make([]string, 201)...) },
		"empty ID":        func(s ListStorage) error { return s.Create(t.Context(), "viewer", "New", "") },
		"long ID":         func(s ListStorage) error { return s.Create(t.Context(), "viewer", "New", strings.Repeat("i", 257)) },
		"duplicate ID":    func(s ListStorage) error { return s.Create(t.Context(), "viewer", "New", "a", "a") },
		"existing manual": func(s ListStorage) error { return s.Create(t.Context(), "viewer", "Queue", "c") },
		"existing smart":  func(s ListStorage) error { return s.Create(t.Context(), "viewer", "Smart") },
		"smart name": func(s ListStorage) error {
			return s.CreateSmart(t.Context(), "viewer", "", PlaylistRule{Sort: "title"})
		},
		"smart query": func(s ListStorage) error {
			return s.CreateSmart(t.Context(), "viewer", "New", PlaylistRule{Query: strings.Repeat("q", 201), Sort: "title"})
		},
		"smart kind": func(s ListStorage) error {
			return s.CreateSmart(t.Context(), "viewer", "New", PlaylistRule{Kind: "unknown", Sort: "title"})
		},
		"smart sort": func(s ListStorage) error {
			return s.CreateSmart(t.Context(), "viewer", "New", PlaylistRule{Sort: "unknown"})
		},
		"smart conflict manual": func(s ListStorage) error {
			return s.CreateSmart(t.Context(), "viewer", "Queue", PlaylistRule{Sort: "title"})
		},
		"smart conflict smart": func(s ListStorage) error {
			return s.CreateSmart(t.Context(), "viewer", "Smart", PlaylistRule{Sort: "title"})
		},
		"missing playlist": func(s ListStorage) error { return s.SetPlaylist(t.Context(), "viewer", "Missing", "a", true) },
		"wrong viewer":     func(s ListStorage) error { return s.SetPlaylist(t.Context(), "other", "Queue", "a", false) },
		"missing order":    func(s ListStorage) error { return s.Order(t.Context(), "viewer", "Missing", nil) },
		"short order":      func(s ListStorage) error { return s.Order(t.Context(), "viewer", "Queue", []string{"a"}) },
		"duplicate order":  func(s ListStorage) error { return s.Order(t.Context(), "viewer", "Queue", []string{"a", "a"}) },
		"unknown order":    func(s ListStorage) error { return s.Order(t.Context(), "viewer", "Queue", []string{"a", "c"}) },
	} {
		t.Run(name, func(t *testing.T) {
			storage := listOperationStorage(func(string, any) error { t.Fatal("rejection persisted state"); return nil })
			before := storage.State()
			if operation(storage) == nil {
				t.Fatal("invalid operation accepted")
			}
			if !reflect.DeepEqual(before, storage.State()) {
				t.Fatal("rejection changed memory")
			}
		})
	}
}

func TestListOperationsPreserveOrderIsolationAndCommitDocuments(t *testing.T) {
	t.Parallel()
	var documents []string
	storage := listOperationStorage(func(path string, _ any) error { documents = append(documents, path); return nil })
	run := func(operation func(context.Context) error, want ...string) {
		t.Helper()
		documents = nil
		if err := operation(t.Context()); err != nil {
			t.Fatal(err)
		}
		if !slices.Equal(documents, want) {
			t.Fatalf("documents=%v want=%v", documents, want)
		}
	}
	run(func(ctx context.Context) error { return storage.Create(ctx, "viewer", " New ", "b", "a") }, "playlists", "order")
	if !slices.Equal((*storage.PlaylistOrder)["viewer:New"], []string{"b", "a"}) {
		t.Fatal("create lost item order or normalization")
	}
	run(func(ctx context.Context) error { return storage.Create(ctx, "viewer", "New") }, "playlists", "order")
	if len((*storage.Playlists)["viewer:New"]) != 2 {
		t.Fatal("empty repeated create cleared playlist")
	}
	run(func(ctx context.Context) error { return storage.SetPlaylist(ctx, "viewer", "New", "b", true) }, "playlists", "order")
	run(func(ctx context.Context) error { return storage.SetPlaylist(ctx, "viewer", "New", "c", true) }, "playlists", "order")
	run(func(ctx context.Context) error { return storage.SetPlaylist(ctx, "viewer", "New", "a", false) }, "playlists", "order")
	if !slices.Equal((*storage.PlaylistOrder)["viewer:New"], []string{"b", "c"}) {
		t.Fatal("membership changed stable order")
	}
	ids := []string{"c", "b"}
	run(func(ctx context.Context) error { return storage.Order(ctx, "viewer", "New", ids) }, "order")
	ids[0] = "changed"
	if !slices.Equal((*storage.PlaylistOrder)["viewer:New"], []string{"c", "b"}) {
		t.Fatal("order aliases input")
	}
	assertListCurationAndDeletion(t, storage, run)
}

func assertListCurationAndDeletion(t *testing.T, storage ListStorage, run func(func(context.Context) error, ...string)) {
	t.Helper()
	run(func(ctx context.Context) error {
		return storage.CreateSmart(ctx, "viewer", " Recent ", PlaylistRule{Kind: "video", Query: "  query  ", Sort: "added"})
	}, "smart")
	if (*storage.Smart)["viewer:Recent"].Query != "query" {
		t.Fatal("smart query was not normalized")
	}
	run(func(ctx context.Context) error { return storage.SetListed(ctx, "viewer", "a", true) }, "values")
	if !(*storage.Values)["viewer:a"] {
		t.Fatal("listed state missing")
	}
	run(func(ctx context.Context) error { return storage.SetListed(ctx, "viewer", "a", false) }, "values")
	if (*storage.Values)["viewer:a"] {
		t.Fatal("listed state remained enabled")
	}
	run(func(ctx context.Context) error { return storage.DeletePlaylist(ctx, "other", "Queue") }, "playlists", "order", "smart")
	if len((*storage.Playlists)["viewer:Queue"]) != 2 {
		t.Fatal("delete crossed viewer scope")
	}
	run(func(ctx context.Context) error { return storage.DeletePlaylist(ctx, "viewer", "New") }, "playlists", "order", "smart")
	if _, ok := (*storage.Playlists)["viewer:New"]; ok {
		t.Fatal("playlist not removed")
	}
	if _, ok := (*storage.PlaylistOrder)["viewer:New"]; ok {
		t.Fatal("order not removed")
	}
	run(func(ctx context.Context) error { return storage.DeletePlaylist(ctx, "viewer", "Recent") }, "playlists", "order", "smart")
	if _, ok := (*storage.Smart)["viewer:Recent"]; ok {
		t.Fatal("smart playlist not removed")
	}
}
