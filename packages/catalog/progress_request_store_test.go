package catalog

import (
	"context"
	"errors"
	"math"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/MikeO7/kinosail/packages/documentdb"
	"github.com/MikeO7/kinosail/packages/library"
)

type progressAuditStub struct {
	actions, titles []string
}

func (*progressAuditStub) ResolveTitle(id string) string {
	if id == "movie" {
		return "Movie"
	}
	return ""
}

func (stub *progressAuditStub) Playback(_ *http.Request, action, _ string, title string, _ float64, _ bool) {
	stub.actions = append(stub.actions, action)
	stub.titles = append(stub.titles, title)
}

func progressRequest(viewer string, owner bool) *http.Request {
	request := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	request.Header.Set("X-Viewer", viewer)
	if owner {
		request.Header.Set("X-Owner", "true")
	}
	return request
}

func testProgressStore(t *testing.T, values map[string]PlaybackState) *RequestProgressStore {
	t.Helper()
	return NewRequestProgressStore("", nil, nil, func(string, any) error { return nil }, func(request *http.Request) ProgressViewer {
		return ProgressViewer{ID: request.Header.Get("X-Viewer"), Owner: request.Header.Get("X-Owner") == "true"}
	}, func(_ *http.Request, item library.Item, state PlaybackState) any {
		return struct {
			ID      string
			Seconds float64
		}{item.ID, state.Seconds}
	}).replaceForTest(values)
}

func (store *RequestProgressStore) replaceForTest(values map[string]PlaybackState) *RequestProgressStore {
	store.Replace(values)
	return store
}

func TestRequestProgressStoreRestoration(t *testing.T) {
	database := new(documentdb.Store)
	loadErr := errors.New("load")
	failed := NewRequestProgressStore(t.TempDir(), database, func(string, any) (bool, error) { return false, loadErr }, func(string, any) error { return nil }, func(*http.Request) ProgressViewer { return ProgressViewer{} }, nil)
	if !errors.Is(failed.Err(), loadErr) || failed.Database() != database {
		t.Fatalf("failed restoration = %v, database=%p", failed.Err(), failed.Database())
	}
	invalid := NewRequestProgressStore(t.TempDir(), nil, func(_ string, target any) (bool, error) {
		*target.(*map[string]PlaybackState) = map[string]PlaybackState{"bad": {Seconds: -1}}
		return true, nil
	}, func(string, any) error { return nil }, func(*http.Request) ProgressViewer { return ProgressViewer{} }, nil)
	if invalid.Err() == nil {
		t.Fatal("invalid restored state was accepted")
	}
	empty := NewRequestProgressStore(t.TempDir(), nil, func(_ string, target any) (bool, error) {
		*target.(*map[string]PlaybackState) = nil
		return true, nil
	}, func(string, any) error { return nil }, func(*http.Request) ProgressViewer { return ProgressViewer{} }, nil)
	if empty.Err() != nil || empty.Len() != 0 {
		t.Fatalf("empty restoration = %v, records=%d", empty.Err(), empty.Len())
	}
	memory := NewRequestProgressStore("", nil, nil, func(string, any) error { return nil }, func(*http.Request) ProgressViewer { return ProgressViewer{} }, nil)
	if memory.Err() != nil || memory.Len() != 0 {
		t.Fatalf("memory store = %v, records=%d", memory.Err(), memory.Len())
	}
}

func TestRequestProgressStoreProjectsViewerState(t *testing.T) { //nolint:cyclop // One projection contract covers every read view below the project ceiling.
	now := time.Now()
	values := map[string]PlaybackState{
		"legacy":       {Seconds: 7},
		"viewer:movie": {Seconds: 15, Updated: now},
		"viewer:done":  {Watched: true, Updated: now.Add(-time.Minute)},
		"viewer:book":  {ReaderPage: 3, Updated: now.Add(-2 * time.Minute)},
	}
	store := testProgressStore(t, values)
	viewer, owner := progressRequest("viewer", false), progressRequest("owner", true)
	if store.Get(viewer, "movie").Seconds != 15 || store.GetFor("owner", true, "legacy").Seconds != 7 || !store.Watched(viewer, "done") {
		t.Fatal("Viewer progress projection failed")
	}
	item := library.Item{ID: "movie", Kind: "video", Title: "Movie"}
	if projected := store.ClientItem(viewer, item).(struct {
		ID      string
		Seconds float64
	}); projected.ID != "movie" || projected.Seconds != 15 {
		t.Fatalf("projected item = %#v", projected)
	}
	if items := store.ClientItems(viewer, []library.Item{item}).([]any); len(items) != 1 {
		t.Fatalf("projected items = %#v", items)
	}
	if store.ReaderPage(viewer, "book", 5) != 3 || store.ReaderPage(viewer, "missing", 5) != 1 || store.ReaderPage(viewer, "book", 2) != 1 {
		t.Fatal("reader page bounds were not applied")
	}
	if active := store.Active(viewer, []library.Item{item}); len(active) != 1 || active[0].ID != "movie" {
		t.Fatalf("active progress = %#v", active)
	}
	if store.Get(owner, "legacy").Seconds != 7 {
		t.Fatal("Owner legacy fallback was not preserved")
	}
	listed := map[string]bool{"viewer:movie": true}
	access := store.BrowseAccess(new(sync.RWMutex), &listed, viewer, func(library.Item) bool { return true })
	if access.ProfileID != "viewer" || access.Owner || access.Progress == nil || access.Listed != &listed || !access.Visible(item) {
		t.Fatalf("browse access = %#v", access)
	}
}

func TestRequestProgressStoreMutationsAndHistory(t *testing.T) { //nolint:cyclop,gocognit // One stateful contract verifies mutation ordering and bounded history below the project ceiling.
	request := progressRequest("viewer", false)
	store := testProgressStore(t, nil)
	persists := 0
	store.SetPersistence(func(string, any) error { persists++; return nil })
	if _, _, changed, err := store.Update("viewer:movie", func(state PlaybackState) (PlaybackState, bool, error) { state.Seconds = 1; return state, true, nil }); err != nil || !changed {
		t.Fatalf("update = %t, %v", changed, err)
	}
	if store.Storage().Values == nil || store.Len() != 1 {
		t.Fatal("storage boundary did not expose committed state")
	}
	if err := store.SetReaderPage(request, "book", 0, 2); err == nil {
		t.Fatal("invalid reader page was accepted")
	}
	if err := store.SetReaderPage(request, "book", 2, 2); err != nil || store.ReaderPage(request, "book", 2) != 2 {
		t.Fatalf("reader progress = %d, %v", store.ReaderPage(request, "book", 2), err)
	}
	if err := store.Dismiss(request, "movie"); err != nil || !store.Get(request, "movie").Dismissed {
		t.Fatalf("dismissed state = %#v, %v", store.Get(request, "movie"), err)
	}
	items := make([]library.Item, 21)
	values := make(map[string]PlaybackState, len(items))
	for index := range items {
		id := string(rune('a' + index))
		items[index] = library.Item{ID: id, Title: id}
		values["viewer:"+id] = PlaybackState{Updated: time.Unix(int64(index+1), 0)}
	}
	store.Replace(values)
	if history := store.History(request, append([]library.Item(nil), items...)); len(history) != 21 || history[0].ID != "u" {
		t.Fatalf("history = %#v", history)
	}
	if recent := store.Recent(request, append([]library.Item(nil), items...)); len(recent) != 20 || recent[0].ID != "u" {
		t.Fatalf("recent = %#v", recent)
	}
	if admin := store.RecentAdmin(items, map[string]string{"viewer": "Viewer"}); len(admin) != 20 || admin[0].Profile != "Viewer" {
		t.Fatalf("admin progress = %#v", admin)
	}
	if persists != 0 {
		t.Fatalf("memory-only store persisted %d times", persists)
	}
}

func TestRequestProgressStoreAuditsAcceptedTransitions(t *testing.T) {
	request := progressRequest("viewer", false)
	store := testProgressStore(t, nil)
	audit := new(progressAuditStub)
	store.SetAuditor(audit)
	if err := store.Set(request, "movie", 10, nil); err != nil {
		t.Fatal(err)
	}
	watched := true
	if accepted, err := store.SetRevision(request, "movie", 10, &watched, "phone", 1); err != nil || !accepted {
		t.Fatalf("completion = %t, %v", accepted, err)
	}
	store.Stop(request, "unknown", 10)
	if len(audit.actions) != 3 || audit.actions[0] != "playback.started" || audit.actions[1] != "playback.completed" || audit.actions[2] != "playback.stopped" || audit.titles[0] != "Movie" || audit.titles[2] != "unknown" {
		t.Fatalf("audit = %#v / %#v", audit.actions, audit.titles)
	}
	store.SetAuditor(nil)
	store.Stop(request, "movie", 10)
}

func TestReaderPositionValidationPreservesSavedSpot(t *testing.T) { //nolint:cyclop // The rejection table verifies one saved reader position remains unchanged.
	request := progressRequest("viewer", false)
	store := testProgressStore(t, nil)
	store.file = filepath.Join(t.TempDir(), "progress.json")
	writes := 0
	store.SetPersistence(func(string, any) error { writes++; return nil })
	if err := store.SetReaderPosition(request, "book", 2, 3, 0.625); err != nil {
		t.Fatal(err)
	}
	saved := store.Get(request, "book")
	before := writes
	for _, input := range []struct {
		page, total int
		offset      float64
	}{
		{0, 3, 0},
		{4, 3, 0},
		{1, 10001, 0},
		{1, 0, 0},
		{2, 3, -0.01},
		{2, 3, 1.01},
		{2, 3, math.NaN()},
		{2, 3, math.Inf(1)},
	} {
		if err := store.SetReaderPosition(request, "book", input.page, input.total, input.offset); err == nil {
			t.Fatalf("accepted %#v", input)
		}
		if store.Get(request, "book") != saved || writes != before {
			t.Fatal("invalid position changed saved state")
		}
	}
	if saved.ReaderPage != 2 || saved.ReaderOffset != 0.625 {
		t.Fatalf("saved %#v", saved)
	}
	if other := store.Get(progressRequest("other", false), "book"); other.ReaderPage != 0 || other.ReaderOffset != 0 {
		t.Fatal("reading position crossed viewers")
	}
	store.SetPersistence(func(string, any) error { return errors.New("disk unavailable") })
	if err := store.SetReaderPosition(request, "book", 3, 3, 1); err == nil || store.Get(request, "book") != saved {
		t.Fatal("failed save changed saved position")
	}
	store.SetPersistence(func(string, any) error { return nil })
	if err := store.SetReaderPage(request, "book", 1, 3); err != nil || store.Get(request, "book").ReaderOffset != 0 {
		t.Fatal("chapter navigation retained the previous offset")
	}
}
