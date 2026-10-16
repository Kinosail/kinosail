package viewing

import (
	"net/http"
	"path/filepath"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/MikeO7/kinosail/packages/catalog"
	"github.com/MikeO7/kinosail/packages/documentdb"
)

func entryCommitStore(b testing.TB, entries int) (CommitStore, *documentdb.Store) {
	b.Helper()
	directory := b.TempDir()
	database, err := documentdb.Open(directory, true, documentdb.PlayerConfig())
	if err != nil {
		b.Fatal(err)
	}
	b.Cleanup(func() { _ = database.Close() })
	values := make(map[string]catalog.PlaybackState, entries)
	for index := range entries {
		values["viewer:item-"+strconv.Itoa(index)] = catalog.PlaybackState{Seconds: float64(index + 1), Updated: time.Unix(int64(index), 0).UTC()}
	}
	if err := database.SaveJSON("progress.json", values); err != nil {
		b.Fatal(err)
	}
	load := func(path string, target any) (bool, error) { return database.LoadJSON(filepath.Base(path), target) }
	persist := func(path string, value any) error { return database.SaveJSON(filepath.Base(path), value) }
	progress := catalog.NewRequestProgressStore(directory, database, load, persist, func(*http.Request) catalog.ProgressViewer { return catalog.ProgressViewer{ID: "viewer"} }, nil)
	if progress.Err() != nil {
		b.Fatal(progress.Err())
	}
	listed, playlists, order, smart := map[string]bool{}, map[string]map[string]bool{}, map[string][]string{}, map[string]catalog.PlaylistRule{}
	paths := catalog.ListPaths{Values: filepath.Join(directory, "lists.json"), Playlists: filepath.Join(directory, "playlists.json"), PlaylistOrder: filepath.Join(directory, "playlist_order.json")}
	var listMutex sync.RWMutex
	return CommitStore{
		Progress:  NewProgressStorage(progress.Storage(), database),
		Lists:     catalog.ListStorage{Values: &listed, Playlists: &playlists, PlaylistOrder: &order, Smart: &smart, Paths: paths, Database: database, Persist: persist},
		ListMutex: &listMutex,
	}, database
}

func BenchmarkViewingCommit20K(b *testing.B) {
	store, _ := entryCommitStore(b, 20_000)
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	previous := make([]catalog.PlaybackState, 5)
	for index := range previous {
		previous[index] = catalog.PlaybackState{Seconds: float64(index + 1), Updated: time.Unix(int64(index), 0).UTC()}
	}
	b.ReportAllocs()
	round := 0
	for b.Loop() {
		round++
		changes := make([]ProgressChange, len(previous))
		for index := range changes {
			next := catalog.PlaybackState{Seconds: float64(round*10 + index), Updated: base.Add(time.Duration(round) * time.Second)}
			changes[index] = ProgressChange{TargetID: "item-" + strconv.Itoa(index), Expected: previous[index], State: next}
			previous[index] = next
		}
		applied, conflicts, _, err := store.Commit(Profile{ID: "viewer"}, changes, []ListChange{{TargetID: "item-" + strconv.Itoa(round), Favorite: true}})
		if err != nil || applied != len(changes) || conflicts != 0 {
			b.Fatalf("applied=%d conflicts=%d err=%v", applied, conflicts, err)
		}
	}
}
