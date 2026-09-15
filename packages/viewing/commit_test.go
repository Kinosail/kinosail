package viewing

import (
	"errors"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/MikeO7/kinosail/packages/catalog"
	"github.com/MikeO7/kinosail/packages/documentdb"
)

func testCommitStore(persist func(string, any) error) (CommitStore, *map[string]catalog.PlaybackState, *map[string]bool, *map[string]map[string]bool, *map[string][]string) {
	progress := map[string]catalog.PlaybackState{}
	listed := map[string]bool{}
	playlists := map[string]map[string]bool{}
	order := map[string][]string{}
	smart := map[string]catalog.PlaylistRule{}
	var progressMutex, persistMutex sync.Mutex
	var listMutex sync.RWMutex
	return CommitStore{
		Progress:  ProgressStorage{Mutex: &progressMutex, PersistMutex: &persistMutex, Values: &progress, File: "progress.json", Persist: persist},
		Lists:     catalog.ListStorage{Values: &listed, Playlists: &playlists, PlaylistOrder: &order, Smart: &smart, Paths: catalog.ListPaths{Values: "listed.json", Playlists: "playlists.json", PlaylistOrder: "order.json"}, Persist: persist},
		ListMutex: &listMutex,
	}, &progress, &listed, &playlists, &order
}

func testCommitChanges() ([]ProgressChange, []ListChange) {
	return []ProgressChange{{TargetID: "movie", State: catalog.PlaybackState{Seconds: 42}}}, []ListChange{{TargetID: "movie", Favorite: true, Playlists: map[string]int{"Queue": 0}}}
}

func TestNewProgressStoragePreservesTheSharedTransaction(t *testing.T) {
	values := map[string]catalog.PlaybackState{}
	mutex, persistMutex := new(sync.Mutex), new(sync.Mutex)
	database := new(documentdb.Store)
	persist := func(string, any) error { return nil }
	storage := NewProgressStorage(catalog.ProgressStorage{Mutex: mutex, PersistMutex: persistMutex, Values: &values, File: "progress.json", Persist: persist}, database)
	if storage.Mutex != mutex || storage.PersistMutex != persistMutex || storage.Values != &values || storage.File != "progress.json" || storage.Persist == nil || storage.Database != database {
		t.Fatalf("progress storage = %#v", storage)
	}
}

func TestCommitStorePublishesAllCategoriesAfterPersistence(t *testing.T) { //nolint:cyclop // One assertion proves the atomic result across every stored category.
	var files []string
	store, progress, listed, playlists, order := testCommitStore(func(file string, _ any) error { files = append(files, file); return nil })
	progressChanges, listChanges := testCommitChanges()
	applied, conflicts, listsApplied, err := store.Commit(Profile{ID: "viewer"}, progressChanges, listChanges)
	if err != nil || applied != 1 || conflicts != 0 || listsApplied != 2 || (*progress)["viewer:movie"].Seconds != 42 || !(*listed)["viewer:movie"] || !(*playlists)["viewer:Queue"]["movie"] || strings.Join((*order)["viewer:Queue"], ",") != "movie" || len(files) != 4 {
		t.Fatalf("result=%d/%d/%d state=%#v/%#v/%#v/%#v files=%#v error=%v", applied, conflicts, listsApplied, *progress, *listed, *playlists, *order, files, err)
	}
	files = nil
	applied, _, listsApplied, err = store.Commit(Profile{ID: "viewer"}, nil, nil)
	if err != nil || applied != 0 || listsApplied != 0 || len(files) != 0 {
		t.Fatalf("no-op=%d/%d files=%#v error=%v", applied, listsApplied, files, err)
	}
}

func TestCommitStoreRollsBackDurableWritesBeforePublishing(t *testing.T) { //nolint:cyclop // One assertion proves failure and rollback boundaries together.
	calls := 0
	store, progress, listed, playlists, _ := testCommitStore(func(string, any) error {
		calls++
		if calls == 4 {
			return errors.New("order failure")
		}
		return nil
	})
	progressChanges, listChanges := testCommitChanges()
	if _, _, _, err := store.Commit(Profile{ID: "viewer"}, progressChanges, listChanges); err == nil {
		t.Fatal("persistence failure accepted")
	}
	if len(*progress) != 0 || len(*listed) != 0 || len(*playlists) != 0 || calls != 7 {
		t.Fatalf("state=%#v/%#v/%#v calls=%d", *progress, *listed, *playlists, calls)
	}
	store, _, _, _, _ = testCommitStore(func(string, any) error { return errors.New("first failure") })
	if _, _, _, err := store.Commit(Profile{ID: "viewer"}, progressChanges, nil); err == nil {
		t.Fatal("first write failure accepted")
	}
	writes := []viewingWrite{{file: "one", persist: func(string, any) error { return errors.New("rollback") }}}
	if err := rollbackViewingWrites(writes, errors.New("cause")); err == nil || !strings.Contains(err.Error(), "rollback") {
		t.Fatalf("rollback error=%v", err)
	}
	if err := rollbackViewingWrites(nil, errors.New("cause")); err == nil || err.Error() != "cause" {
		t.Fatalf("empty rollback=%v", err)
	}
}

func TestCommitStoreUsesOneDocumentTransaction(t *testing.T) {
	config := documentdb.PlayerConfig()
	database, err := documentdb.Open(t.TempDir(), true, config)
	if err != nil {
		t.Fatal(err)
	}
	store, progress, listed, _, _ := testCommitStore(func(string, any) error { t.Fatal("file persistence used"); return nil })
	store.Progress.Database, store.Lists.Database = database, database
	store.Progress.File = filepath.Join(t.TempDir(), "progress.json")
	store.Lists.Paths = catalog.ListPaths{Values: filepath.Join(t.TempDir(), "lists.json"), Playlists: filepath.Join(t.TempDir(), "playlists.json"), PlaylistOrder: filepath.Join(t.TempDir(), "playlist_order.json")}
	progressChanges, listChanges := testCommitChanges()
	if _, _, _, err := store.Commit(Profile{ID: "viewer"}, progressChanges, listChanges); err != nil || len(*progress) != 1 || len(*listed) != 1 {
		t.Fatalf("state=%#v/%#v error=%v", *progress, *listed, err)
	}
	if err := database.Close(); err != nil {
		t.Fatal(err)
	}
	store, progress, listed, _, _ = testCommitStore(nil)
	store.Progress.Database, store.Lists.Database = database, database
	if _, _, _, err := store.Commit(Profile{ID: "viewer"}, progressChanges, listChanges); err == nil || len(*progress) != 0 || len(*listed) != 0 {
		t.Fatalf("closed database state=%#v/%#v error=%v", *progress, *listed, err)
	}
}

func TestCommitStoreSkipsEmptyDocumentKeys(t *testing.T) {
	config := documentdb.PlayerConfig()
	database, err := documentdb.Open(t.TempDir(), true, config)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	store, progress, listed, _, _ := testCommitStore(nil)
	store.Progress.Database, store.Lists.Database = database, database
	store.Progress.File = ""
	store.Lists.Paths = catalog.ListPaths{Values: "lists.json", Playlists: "playlists.json", PlaylistOrder: "playlist_order.json"}
	progressChanges, listChanges := testCommitChanges()
	if applied, _, _, err := store.Commit(Profile{ID: "viewer"}, progressChanges, listChanges); err != nil || applied != 1 || len(*progress) != 1 || !(*listed)["viewer:movie"] {
		t.Fatalf("state=%#v/%#v applied=%d error=%v", *progress, *listed, applied, err)
	}
	persisted := make(map[string]bool)
	if found, err := database.LoadJSON("lists.json", &persisted); err != nil || !found || !persisted["viewer:movie"] {
		t.Fatalf("persisted lists=%#v found=%t error=%v", persisted, found, err)
	}
}

func TestCommitStoreSkipsUnconfiguredFileWrites(t *testing.T) {
	persists := 0
	store, progress, _, _, _ := testCommitStore(func(string, any) error { persists++; return nil })
	store.Progress.File = ""
	progressChanges, listChanges := testCommitChanges()
	if applied, _, _, err := store.Commit(Profile{ID: "viewer"}, progressChanges, listChanges); err != nil || applied != 1 || len(*progress) != 1 || persists != 3 {
		t.Fatalf("state=%#v applied=%d persists=%d error=%v", *progress, applied, persists, err)
	}
}
