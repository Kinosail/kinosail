package viewing

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/MikeO7/kinosail/packages/catalog"
	"github.com/MikeO7/kinosail/packages/library"
)

func TestRunSyncPullsAndPersistsSourceState(t *testing.T) { //nolint:cyclop // One reconciliation assertion covers its coupled durable result.
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = writer.Write([]byte(`{"MediaContainer":{"Directory":[]}}`))
	}))
	defer server.Close()
	config := testManagerConfig()
	config.Client = server.Client()
	persists := 0
	config.Persist = func(string, any) error { persists++; return nil }
	manager := newTestManager(t.TempDir(), config)
	manager.syncs["sync"] = Sync{ID: "sync", Source: "plex", URL: server.URL, Token: "secret", ProfileID: "viewer", Interval: "1h", Seen: map[string]Observation{}}
	view, err := manager.RunSync(t.Context(), "sync")
	if err != nil || view.ID != "sync" || view.LastRun == "" || view.NextRun == "" || persists != 1 || manager.syncs["sync"].Running || manager.syncs["sync"].LastError != "" {
		t.Fatalf("view=%#v state=%#v persists=%d error=%v", view, manager.syncs, persists, err)
	}

	manager.config.Client = &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) { return nil, errors.New("network secret") })}
	view, err = manager.RunSync(t.Context(), "sync")
	if err == nil || view.LastError == "" || !strings.Contains(manager.syncs["sync"].LastError, "unavailable") {
		t.Fatalf("view=%#v state=%#v error=%v", view, manager.syncs, err)
	}

	manager.config.Client = server.Client()
	manager.config.Persist = func(string, any) error { return errors.New("disk") }
	if _, err := manager.RunSync(t.Context(), "sync"); err == nil {
		t.Fatal("sync persistence failure accepted")
	}
}

func TestApplyPendingKeepsDurableRecoveryUntilFinalized(t *testing.T) { //nolint:cyclop // One recovery assertion covers each atomic failure point.
	config := testManagerConfig()
	manager := newTestManager(t.TempDir(), config)
	pending := &SyncPending{Summary: Summary{Importable: 1}, Changes: []ProgressChange{{TargetID: "movie"}}, ListChanges: []ListChange{{TargetID: "movie", Favorite: true}}}
	sync := Sync{ID: "sync", ProfileID: "viewer", Interval: "1h", Pending: pending}
	manager.syncs[sync.ID] = sync
	result, err := manager.applyPending(sync)
	if err != nil || result.Applied != 1 || result.ListsApplied != 1 || manager.syncs[sync.ID].Pending != nil {
		t.Fatalf("result=%#v state=%#v error=%v", result, manager.syncs, err)
	}
	manager.syncs[sync.ID] = sync
	if _, err := manager.RunSync(t.Context(), sync.ID); err != nil || manager.syncs[sync.ID].Pending != nil {
		t.Fatalf("pending run state=%#v error=%v", manager.syncs, err)
	}

	missing := sync
	missing.ProfileID = "missing"
	if _, err := manager.applyPending(missing); err == nil {
		t.Fatal("missing profile accepted")
	}
	manager.syncs[sync.ID] = sync
	manager.config.Commit = func(Profile, []ProgressChange, []ListChange) (int, int, int, error) {
		return 0, 1, 0, errors.New("commit")
	}
	result, err = manager.applyPending(sync)
	if err == nil || result.Conflicts != 1 || manager.syncs[sync.ID].Pending == nil {
		t.Fatalf("result=%#v state=%#v error=%v", result, manager.syncs, err)
	}
	manager.config.Commit = testManagerConfig().Commit
	manager.config.Persist = func(string, any) error { return errors.New("disk") }
	if _, err := manager.applyPending(sync); err == nil || manager.syncs[sync.ID].Pending == nil {
		t.Fatalf("state=%#v error=%v", manager.syncs, err)
	}
}

func TestPullClassifiesEveryRecurringOutcome(t *testing.T) { //nolint:cyclop,funlen // One reconciliation batch proves observed identity and optimistic conflict rules.
	items := []library.Item{
		{ID: "same", Kind: "video", Title: "Same", Year: "2024"},
		{ID: "conflict", Kind: "video", Title: "Conflict", Year: "2024"},
		{ID: "ready", Kind: "video", Title: "Ready", Year: "2024"},
		{ID: "stable", Kind: "video", Title: "Renamed", Year: "2024"},
		{ID: "duplicate-a", Kind: "video", Title: "Duplicate", Year: "2024"},
		{ID: "duplicate-b", Kind: "video", Title: "Duplicate", Year: "2024"},
	}
	progress := map[string]catalog.PlaybackState{
		"same":     {Seconds: 5},
		"conflict": {Seconds: 1, Updated: time.Now()},
	}
	var changes []ProgressChange
	config := testManagerConfig()
	config.Snapshot = func() ([]library.Item, error) { return items, nil }
	config.Progress = func(_ Profile, id string) catalog.PlaybackState { return progress[id] }
	config.ImportProgress = func(_ Profile, values []ProgressChange) (int, int, error) {
		changes = append(changes, values...)
		return len(values), 1, nil
	}
	manager := newTestManager("", config)
	now := time.Now().UTC()
	unchanged := Activity{SourceID: "observed", Kind: "movie", Title: "Whatever", Seconds: 8}
	stable := Activity{SourceID: "stable-source", Kind: "movie", Title: "No longer matches", Seconds: 9}
	sync := Sync{ProfileID: "viewer", Seen: map[string]Observation{
		ActivityKey(unchanged): {TargetID: "same", Signature: Signature(unchanged)},
		ActivityKey(stable):    {TargetID: "stable", Signature: "old"},
	}}
	activities := []Activity{
		{SourceID: "inactive", Kind: "movie", Title: "Missing"},
		unchanged,
		{SourceID: "ambiguous", Kind: "movie", Title: "Duplicate", Year: "2024", Seconds: 1},
		{SourceID: "missing", Kind: "movie", Title: "Missing", Year: "2024", Seconds: 1},
		{SourceID: "same", Kind: "movie", Title: "Same", Year: "2024", Seconds: 5},
		{SourceID: "conflict", Kind: "movie", Title: "Conflict", Year: "2024", Seconds: 9, Updated: time.Now().Add(-time.Hour)},
		{SourceID: "ready", Kind: "movie", Title: "Ready", Year: "2024", Seconds: 7},
		stable,
	}
	result, seen, err := manager.pull(sync, activities, now)
	if err != nil || result.SourceItems != 8 || result.Activity != 7 || result.Matched != 4 || result.Ambiguous != 1 || result.Unmatched != 1 || result.Unchanged != 2 || result.Conflicts != 2 || result.Importable != 2 || result.Applied != 2 || len(changes) != 2 || seen[ActivityKey(stable)].TargetID != "stable" || changes[1].State.Updated != now {
		t.Fatalf("result=%#v changes=%#v seen=%#v error=%v", result, changes, seen, err)
	}
	if item, found := findLibraryItem(items, "ready"); !found || item.ID != "ready" {
		t.Fatal("existing item was not found")
	}
	if _, found := findLibraryItem(items, "missing"); found {
		t.Fatal("missing item was found")
	}
}

func TestPullFailsBeforeImportWhenPrivateAdaptersFail(t *testing.T) {
	config := testManagerConfig()
	manager := newTestManager("", config)
	sync := Sync{ProfileID: "missing", Seen: map[string]Observation{}}
	if _, _, err := manager.pull(sync, nil, time.Now()); err == nil {
		t.Fatal("missing profile accepted")
	}
	sync.ProfileID = "viewer"
	manager.config.Snapshot = func() ([]library.Item, error) { return nil, errors.New("index") }
	if _, _, err := manager.pull(sync, nil, time.Now()); err == nil {
		t.Fatal("missing library accepted")
	}
	manager.config.Snapshot = config.Snapshot
	manager.config.ImportProgress = func(Profile, []ProgressChange) (int, int, error) { return 0, 0, errors.New("progress") }
	result, _, err := manager.pull(sync, []Activity{{SourceID: "ready", Kind: "movie", Title: "Arrival", Year: "2016", Seconds: 1}}, time.Now())
	if err == nil || result.Importable != 1 {
		t.Fatalf("result=%#v error=%v", result, err)
	}
}

func TestRunDueSkipsUnavailableAndFutureSyncs(t *testing.T) {
	manager := newTestManager("", testManagerConfig())
	future := time.Now().Add(time.Hour)
	manager.syncs["a-running"] = Sync{ID: "a-running", Running: true}
	manager.syncs["b-queued"] = Sync{ID: "b-queued", Queued: true}
	manager.syncs["c-future"] = Sync{ID: "c-future", NextRun: future}
	manager.syncSlots <- struct{}{}
	manager.syncSlots <- struct{}{}
	manager.syncs["z-due"] = Sync{ID: "z-due"}
	manager.RunDue(t.Context(), time.Now())
	if manager.syncDeferred.Load() != 1 || manager.syncs["a-running"].Queued || manager.syncs["b-queued"].Running || manager.syncs["c-future"].Queued {
		t.Fatalf("state=%#v deferred=%d", manager.syncs, manager.syncDeferred.Load())
	}
	<-manager.syncSlots
	<-manager.syncSlots
}

func TestManagerLoadErrorAndScheduleTimer(t *testing.T) {
	if initialSyncDelay != 250*time.Millisecond {
		t.Fatalf("initial delay=%v", initialSyncDelay)
	}
	dataDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dataDir, "viewing_imports.json"), []byte(`{`), 0o600); err != nil {
		t.Fatal(err)
	}
	manager := newTestManager(dataDir, testManagerConfig())
	if manager.loadErr == nil || len(manager.syncs) != 0 {
		t.Fatalf("state=%#v error=%v", manager.syncs, manager.loadErr)
	}
	valid := `{"ABCDEFGHIJKLMNOP234567":{"id":"ABCDEFGHIJKLMNOP234567","source":"plex","url":"https://plex.example/","token":"secret","profileId":"viewer","interval":"1h","lastResult":{}}}`
	if err := os.WriteFile(filepath.Join(dataDir, "viewing_imports.json"), []byte(valid), 0o600); err != nil {
		t.Fatal(err)
	}
	manager = newTestManager(dataDir, testManagerConfig())
	if manager.loadErr != nil || manager.syncs["ABCDEFGHIJKLMNOP234567"].URL != "https://plex.example" {
		t.Fatalf("state=%#v error=%v", manager.syncs, manager.loadErr)
	}
	ctx, cancel := context.WithCancel(t.Context())
	_ = NewManager(ctx, "", testManagerConfig())
	time.Sleep(275 * time.Millisecond)
	cancel()
}

func TestWithoutViewingPreviewKeepsOtherCredentials(t *testing.T) {
	previews := map[string]Preview{"remove": {ID: "remove"}, "keep": {ID: "keep"}}
	result := withoutViewingPreview(previews, "remove")
	if len(result) != 1 || result["keep"].ID != "keep" || len(previews) != 2 {
		t.Fatalf("result=%#v source=%#v", result, previews)
	}
}
