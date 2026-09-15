package viewing

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/MikeO7/kinosail/packages/catalog"
	"github.com/MikeO7/kinosail/packages/library"
)

func testManagerConfig() Config {
	profiles := map[string]Profile{"viewer": {ID: "viewer", Name: "Viewer"}}
	return Config{
		Client:  http.DefaultClient,
		Persist: func(string, any) error { return nil },
		Snapshot: func() ([]library.Item, error) {
			return []library.Item{{ID: "movie", Kind: "video", Title: "Arrival", Year: "2016"}}, nil
		},
		Profile:        func(id string) (Profile, bool) { profile, found := profiles[id]; return profile, found },
		Progress:       func(Profile, string) catalog.PlaybackState { return catalog.PlaybackState{} },
		ListAdditions:  func(Profile, string, bool, map[string]int) int { return 0 },
		ImportProgress: func(_ Profile, changes []ProgressChange) (int, int, error) { return len(changes), 0, nil },
		Commit: func(_ Profile, progress []ProgressChange, lists []ListChange) (int, int, int, error) {
			return len(progress), 0, len(lists), nil
		},
	}
}

func newTestManager(dataDir string, config Config) *Manager { //nolint:staticcheck // A nil context intentionally disables the background scheduler in deterministic tests.
	return NewManager(nil, dataDir, config) //nolint:staticcheck // A nil context intentionally disables the background scheduler.
}

func testPreview() Preview {
	return Preview{
		ID: "preview", ProfileID: "viewer", ExpiresAt: time.Now().Add(time.Hour),
		Input:   Input{Source: "plex", URL: "https://plex.example", Token: "secret", ProfileID: "viewer"},
		Summary: Summary{Importable: 1}, Activities: []Activity{
			{SourceID: "source", Kind: "movie", Title: "Arrival", Year: "2016", Path: "/source.mkv", Seconds: 42},
			{SourceID: "watched", Kind: "movie", Title: "Arrival", Year: "2016", Path: "/watched.mkv", Watched: true},
			{SourceID: "inactive", Kind: "movie", Title: "Arrival", Year: "2016", Path: "/inactive.mkv"},
		},
		Changes:     []ProgressChange{{TargetID: "movie", State: catalog.PlaybackState{Seconds: 42}}},
		ListChanges: []ListChange{{TargetID: "movie", Favorite: true}},
	}
}

func TestManagerValidatesInputsBeforeSourceWork(t *testing.T) { //nolint:cyclop // Each independent input bound must fail before network or storage callbacks.
	config := testManagerConfig()
	manager := newTestManager("", config)
	valid := Input{Source: "plex", URL: "https://plex.example", Token: "secret", ProfileID: "viewer"}
	changes := map[string]func(*Input){
		"source":           func(input *Input) { input.Source = "other" },
		"missing host":     func(input *Input) { input.URL = "https:///path" },
		"bad scheme":       func(input *Input) { input.URL = "file:///tmp/source" },
		"userinfo":         func(input *Input) { input.URL = "https://user@plex.example" },
		"query":            func(input *Input) { input.URL += "?token=secret" },
		"fragment":         func(input *Input) { input.URL += "#fragment" },
		"long URL":         func(input *Input) { input.URL = "https://" + strings.Repeat("a", 2048) },
		"token":            func(input *Input) { input.Token = "" },
		"long token":       func(input *Input) { input.Token = strings.Repeat("a", 4097) },
		"long source user": func(input *Input) { input.SourceUser = strings.Repeat("a", 513) },
		"profile":          func(input *Input) { input.ProfileID = "missing" },
	}
	for name, change := range changes {
		t.Run(name, func(t *testing.T) {
			input := valid
			change(&input)
			if _, _, err := manager.validate(input); err == nil {
				t.Fatal("invalid input accepted")
			}
		})
	}
	input, profile, err := manager.validate(Input{Source: " JELLYFIN ", URL: " https://source.example/root/ ", Token: " secret ", SourceUser: " user ", ProfileID: " viewer "})
	if err != nil || input.Source != "jellyfin" || input.URL != "https://source.example/root" || input.Token != "secret" || input.SourceUser != "user" || profile.ID != "viewer" {
		t.Fatalf("input=%#v profile=%#v error=%v", input, profile, err)
	}
	boundaryURL := "https://source.example/" + strings.Repeat("a", 2048-len("https://source.example/"))
	if _, _, err := manager.validate(Input{Source: "plex", URL: boundaryURL, Token: strings.Repeat("t", 4096), SourceUser: strings.Repeat("u", 512), ProfileID: "viewer"}); err != nil {
		t.Fatalf("valid boundary input rejected: %v", err)
	}
	if previewLimit != 100 || previewWorkers != 2 || syncWorkers != 2 {
		t.Fatalf("worker limits=%d/%d/%d", previewLimit, previewWorkers, syncWorkers)
	}
}

func TestManagerPreviewBuildApplyAndMetrics(t *testing.T) { //nolint:cyclop // One lifecycle assertion covers the coupled manager projections.
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		calls++
		if calls > 10 {
			http.Error(writer, "pagination runaway", http.StatusLoopDetected)
			return
		}
		if request.URL.Path == "/Items" {
			_, _ = writer.Write([]byte(`{"TotalRecordCount":1,"Items":[{"Id":"source","Name":"Arrival","Type":"Movie","ProductionYear":2016,"UserData":{"PlaybackPositionTicks":420000000}}]}`))
			return
		}
		_, _ = writer.Write([]byte(`{"TotalRecordCount":0,"Items":[]}`))
	}))
	defer server.Close()
	config := testManagerConfig()
	config.Client = server.Client()
	manager := newTestManager("", config)
	preview, err := manager.Preview(t.Context(), Input{Source: "jellyfin", URL: server.URL, Token: "secret", SourceUser: "user", ProfileID: "viewer"})
	if err != nil || preview.Summary.Importable != 1 {
		t.Fatalf("preview=%#v error=%v", preview, err)
	}
	result, err := manager.Apply(preview.ID)
	if err != nil || result.Applied != 1 {
		t.Fatalf("result=%#v error=%v", result, err)
	}
	built := manager.Build(Input{Source: "plex"}, Profile{ID: "viewer", Name: "Viewer"}, testPreview().Activities, []library.Item{{ID: "movie", Kind: "video", Title: "Arrival", Year: "2016"}})
	if built.ProfileName != "Viewer" || built.Source != "plex" {
		t.Fatalf("built=%#v", built)
	}
	manager.syncSlots <- struct{}{}
	manager.syncDeferred.Add(2)
	if active, deferred := manager.Metrics(); active != 1 || deferred != 2 {
		t.Fatalf("metrics=%d/%d", active, deferred)
	}
	<-manager.syncSlots
}

func TestManagerRestoresAndSchedulesDurableState(t *testing.T) {
	config := testManagerConfig()
	dataDir := t.TempDir()
	manager := newTestManager(dataDir, config)
	manager.previews["old"] = Preview{ID: "old", ExpiresAt: time.Now().Add(-time.Second)}
	manager.RunDue(t.Context(), time.Now())
	if len(manager.previews) != 0 {
		t.Fatal("expired preview remained")
	}
	manager = newTestManager(dataDir, config)
	if manager.loadErr != nil || len(manager.Syncs()) != 0 {
		t.Fatalf("restored=%#v error=%v", manager.Syncs(), manager.loadErr)
	}
	ctx, cancel := context.WithCancel(t.Context())
	_ = NewManager(ctx, "", config)
	cancel()
	time.Sleep(time.Millisecond)
}

func TestCreateSyncIsAtomicAcrossPersistenceAndCommit(t *testing.T) { //nolint:cyclop,funlen,gocognit // Every failure point keeps a recoverable preview or durable pending batch.
	config := testManagerConfig()
	manager := newTestManager(t.TempDir(), config)
	if _, err := manager.CreateSync("missing", "bad"); err == nil {
		t.Fatal("invalid interval accepted")
	}
	if _, err := manager.CreateSync("missing", "1h"); err == nil {
		t.Fatal("missing preview accepted")
	}
	preview := testPreview()
	manager.previews[preview.ID] = preview
	missing := config
	missing.Profile = func(string) (Profile, bool) { return Profile{}, false }
	missingManager := newTestManager(t.TempDir(), missing)
	missingManager.previews[preview.ID] = preview
	if _, err := missingManager.CreateSync(preview.ID, "1h"); err == nil {
		t.Fatal("missing profile accepted")
	}

	persistFailure := newTestManager(t.TempDir(), config)
	persistFailure.previews[preview.ID] = preview
	persistFailure.config.Persist = func(string, any) error { return errors.New("persist") }
	if _, err := persistFailure.CreateSync(preview.ID, "1h"); err == nil || len(persistFailure.syncs) != 0 || len(persistFailure.previews) != 1 {
		t.Fatalf("persist failure state=%#v/%#v error=%v", persistFailure.syncs, persistFailure.previews, err)
	}

	commitFailure := newTestManager(t.TempDir(), config)
	commitFailure.previews[preview.ID] = preview
	commitFailure.config.Commit = func(Profile, []ProgressChange, []ListChange) (int, int, int, error) {
		return 0, 0, 0, errors.New("commit")
	}
	if _, err := commitFailure.CreateSync(preview.ID, "1h"); err == nil || len(commitFailure.syncs) != 0 || len(commitFailure.previews) != 1 {
		t.Fatalf("commit failure state=%#v/%#v error=%v", commitFailure.syncs, commitFailure.previews, err)
	}

	rollbackFailure := newTestManager(t.TempDir(), config)
	rollbackFailure.previews[preview.ID] = preview
	calls := 0
	rollbackFailure.config.Persist = func(string, any) error {
		calls++
		if calls == 2 {
			return errors.New("rollback")
		}
		return nil
	}
	rollbackFailure.config.Commit = commitFailure.config.Commit
	if _, err := rollbackFailure.CreateSync(preview.ID, "1h"); err == nil || len(rollbackFailure.syncs) != 1 {
		t.Fatalf("rollback state=%#v error=%v", rollbackFailure.syncs, err)
	}

	finalizeFailure := newTestManager(t.TempDir(), config)
	finalizeFailure.previews[preview.ID] = preview
	calls = 0
	finalizeFailure.config.Persist = func(string, any) error {
		calls++
		if calls == 2 {
			return errors.New("finalize")
		}
		return nil
	}
	view, err := finalizeFailure.CreateSync(preview.ID, "1h")
	if err != nil || finalizeFailure.syncs[view.ID].Pending == nil || len(finalizeFailure.previews) != 0 || !strings.Contains(view.LastError, "retry") {
		t.Fatalf("pending=%#v view=%#v error=%v", finalizeFailure.syncs, view, err)
	}

	success := newTestManager(t.TempDir(), config)
	var saved []map[string]Sync
	success.config.Persist = func(_ string, value any) error {
		saved = append(saved, CloneSyncs(value.(map[string]Sync)))
		return nil
	}
	success.config.Commit = func(_ Profile, progress []ProgressChange, lists []ListChange) (int, int, int, error) {
		return len(progress), 2, len(lists), nil
	}
	success.previews[preview.ID] = preview
	view, err = success.CreateSync(preview.ID, "15m")
	_, sawActive := success.syncs[view.ID].Seen[ActivityKey(preview.Activities[0])]
	_, sawWatched := success.syncs[view.ID].Seen[ActivityKey(preview.Activities[1])]
	_, sawInactive := success.syncs[view.ID].Seen[ActivityKey(preview.Activities[2])]
	if err != nil || view.ID == "" || view.ProfileName != "Viewer" || view.LastResult.Conflicts != 2 || view.LastResult.Applied != 1 || view.LastResult.ListsApplied != 1 || len(saved) != 2 || saved[0][view.ID].LastResult.Applied != 1 || saved[0][view.ID].LastResult.ListsApplied != 0 || len(success.previews) != 0 || len(success.syncs) != 1 || success.syncs[view.ID].Pending != nil || len(success.syncs[view.ID].Seen) != 2 || !sawActive || !sawWatched || sawInactive {
		t.Fatalf("view=%#v state=%#v error=%v", view, success.syncs, err)
	}
	if syncIntervals["15m"] != 15*time.Minute || syncIntervals["1h"] != time.Hour || syncIntervals["6h"] != 6*time.Hour || syncIntervals["24h"] != 24*time.Hour {
		t.Fatalf("intervals=%#v", syncIntervals)
	}
}
