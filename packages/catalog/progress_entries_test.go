package catalog

import (
	"net/http"
	"path/filepath"
	"testing"

	"github.com/MikeO7/kinosail/packages/documentdb"
)

func TestProgressEntryPersistenceRestoresAcceptedState(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()
	database, err := documentdb.Open(directory, true, documentdb.Config{Documents: []string{"progress.json"}})
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	viewer := func(*http.Request) ProgressViewer { return ProgressViewer{ID: "viewer"} }
	store := NewRequestProgressStore(directory, database, progressDatabaseLoader(database), nil, viewer, nil)
	if store.Err() != nil {
		t.Fatal(store.Err())
	}
	request := progressRequest("viewer", false)
	if err := store.Set(request, "movie", 23, nil); err != nil {
		t.Fatal(err)
	}
	restored := NewRequestProgressStore(directory, database, progressDatabaseLoader(database), nil, viewer, nil)
	if restored.Err() != nil || restored.Get(request, "movie").Seconds != 23 {
		t.Fatalf("restored=%#v err=%v", restored.Get(request, "movie"), restored.Err())
	}
}

func TestProgressEntryCommitFailurePreservesMemory(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()
	database, err := documentdb.Open(directory, true, documentdb.Config{Documents: []string{"progress.json"}})
	if err != nil {
		t.Fatal(err)
	}
	store := NewRequestProgressStore(directory, database, progressDatabaseLoader(database), nil, func(*http.Request) ProgressViewer { return ProgressViewer{ID: "viewer"} }, nil)
	if store.Err() != nil {
		t.Fatal(store.Err())
	}
	if err := database.Close(); err != nil {
		t.Fatal(err)
	}
	request := progressRequest("viewer", false)
	if err := store.Set(request, "movie", 23, nil); err == nil || store.Len() != 0 {
		t.Fatalf("failed commit=%v count=%d", err, store.Len())
	}
}

func progressDatabaseLoader(database *documentdb.Store) func(string, any) (bool, error) {
	return func(path string, target any) (bool, error) { return database.LoadJSON(filepath.Base(path), target) }
}
