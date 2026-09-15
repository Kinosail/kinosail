package viewing

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/MikeO7/kinosail/packages/catalog"
	"github.com/MikeO7/kinosail/packages/library"
)

func TestMatchUsesConservativeIdentityOrder(t *testing.T) { //nolint:cyclop // One table proves every intentional match fallback.
	items := []library.Item{
		{ID: "provider", Kind: "video", Title: "Different", Year: "2020", ProviderIDs: map[string]string{"imdb": "tt123"}},
		{ID: "movie-a", Kind: "video", Title: "Arrival", Year: "2016", Path: "/a/Arrival.mkv"},
		{ID: "movie-b", Kind: "video", Title: "Arrival", Year: "2016", Path: `C:\b\Arrival.other.mkv`},
		{ID: "episode", Kind: "video", Title: "Pilot", Show: "The Show", Season: 1, Episode: 2},
	}
	cases := []struct {
		activity           Activity
		id, status, reason string
	}{
		{Activity{Kind: "movie", ProviderIDs: map[string]string{"imdb": "TT123"}}, "provider", "matched", "provider identifier"},
		{Activity{Kind: "movie", Title: "Arrival", Year: "2016", Path: "/source/Arrival.mkv"}, "movie-a", "matched", "Movie title and year and media filename"},
		{Activity{Kind: "movie", Title: "Arrival", Year: "2016"}, "", "ambiguous", "Movie title and year matched multiple Kinosail items"},
		{Activity{Kind: "episode", Show: "The-Show", Season: 1, Episode: 2}, "episode", "matched", "Show, season, and Episode"},
		{Activity{Kind: "movie", Title: "Arrival"}, "", "unmatched", "no Kinosail Library item matched"},
		{Activity{Kind: "unsupported", Title: "Arrival", Year: "2016"}, "", "ambiguous", "Movie title and year matched multiple Kinosail items"},
	}
	for _, test := range cases {
		item, status, reason := Match(test.activity, items)
		if item.ID != test.id || status != test.status || reason != test.reason {
			t.Errorf("Match(%#v) = %q, %q, %q", test.activity, item.ID, status, reason)
		}
	}
	if !Conflict(catalog.PlaybackState{Updated: time.Now()}, catalog.PlaybackState{}) || Conflict(catalog.PlaybackState{}, catalog.PlaybackState{}) {
		t.Fatal("conflict ordering is incorrect")
	}
	if !SameState(catalog.PlaybackState{Seconds: 2, Watched: true}, catalog.PlaybackState{Seconds: 2.4, Watched: true}) || SameState(catalog.PlaybackState{Seconds: 2}, catalog.PlaybackState{Seconds: 2.5}) || SameState(catalog.PlaybackState{Seconds: 2}, catalog.PlaybackState{Seconds: 3}) {
		t.Fatal("state equality tolerance is incorrect")
	}
	if normalizedText(" A-b_2! ") != "ab2" {
		t.Fatal("identity normalization changed")
	}
	if sameKind(Activity{Kind: "episode"}, library.Item{Kind: "video"}) || sameKind(Activity{Kind: "movie"}, library.Item{Kind: "audio"}) || sameKind(Activity{Kind: "movie"}, library.Item{Kind: "video", Show: "Show"}) {
		t.Fatal("media kind classification changed")
	}
	baseEpisode := Activity{Kind: "episode", Show: "Show", Season: 1, Episode: 2}
	for _, item := range []library.Item{{Kind: "video", Show: "Other", Season: 1, Episode: 2}, {Kind: "video", Show: "Show", Season: 2, Episode: 2}, {Kind: "video", Show: "Show", Season: 1, Episode: 3}} {
		if sameIdentity(baseEpisode, item) {
			t.Fatalf("episode identity accepted: %#v", item)
		}
	}
}

func TestActivityIdentifiersAndLabelsAreStable(t *testing.T) {
	episode := Activity{Kind: "episode", Show: "Show", Title: "Pilot", Season: 1, Episode: 2, Watched: true, Seconds: 1.2345, ProviderIDs: map[string]string{"tmdb": "9", "imdb": "TT1"}}
	if ActivityKey(episode) != "episode|imdb:tt1" || Signature(episode) != "true|1.234" || Label(episode) != "Show · S01E02 · Pilot" {
		t.Fatalf("key=%q signature=%q label=%q", ActivityKey(episode), Signature(episode), Label(episode))
	}
	movie := Activity{Kind: "movie", Title: "Movie", Year: "2024", Path: `C:\Media\Movie.mkv`}
	if ActivityKey(movie) != "movie|path:movie.mkv" || Label(movie) != "Movie · 2024" {
		t.Fatalf("movie key=%q label=%q", ActivityKey(movie), Label(movie))
	}
	plain := Activity{Kind: "movie", Title: "Movie"}
	if ActivityKey(plain) != "movie|movie||0|0" || Label(plain) != "Movie" {
		t.Fatalf("plain key=%q label=%q", ActivityKey(plain), Label(plain))
	}
}

func TestBuildPreviewClassifiesWithoutSideEffects(t *testing.T) { //nolint:cyclop // One preview proves all classification outcomes and callback boundaries.
	items := []library.Item{
		{ID: "ready", Kind: "video", Title: "Ready", Year: "2024"},
		{ID: "same", Kind: "video", Title: "Same", Year: "2024"},
		{ID: "conflict", Kind: "video", Title: "Conflict", Year: "2024"},
		{ID: "duplicate-a", Kind: "video", Title: "Duplicate", Year: "2024"},
		{ID: "duplicate-b", Kind: "video", Title: "Duplicate", Year: "2024"},
	}
	activities := []Activity{
		{Kind: "movie", Title: "Ignored", Year: "2024"},
		{Kind: "movie", Title: "Ready", Year: "2024", Seconds: 12, Favorite: true, Playlists: map[string]int{"Queue": 1}},
		{Kind: "movie", Title: "Same", Year: "2024", Seconds: 5},
		{Kind: "movie", Title: "Conflict", Year: "2024", Seconds: 9},
		{Kind: "movie", Title: "Duplicate", Year: "2024", Watched: true},
		{Kind: "movie", Title: "Missing", Year: "2024", Favorite: true},
		{Kind: "movie", Title: "Ready", Year: "2024", Favorite: true},
	}
	progressReads, listReads := 0, 0
	plan := BuildPreview(PreviewInput{
		Activities: activities, Items: items,
		Progress: func(id string) catalog.PlaybackState {
			progressReads++
			switch id {
			case "same":
				return catalog.PlaybackState{Seconds: 5}
			case "conflict":
				return catalog.PlaybackState{Seconds: 2, Updated: time.Now()}
			default:
				return catalog.PlaybackState{}
			}
		},
		ListAdditions: func(id string, favorite bool, playlists map[string]int) int {
			listReads++
			if id == "ready" {
				return 2
			}
			return 0
		},
	})
	if plan.Summary != (Summary{SourceItems: 7, Activity: 4, Matched: 4, Importable: 2, Unchanged: 1, Ambiguous: 1, Unmatched: 1, Conflicts: 1, Favorites: 3, PlaylistItems: 1}) {
		t.Fatalf("summary = %#v", plan.Summary)
	}
	if len(plan.Changes) != 1 || len(plan.ListChanges) != 2 || progressReads != 3 || listReads != 4 || plan.Items[0].Playlists[0] != "Queue" {
		t.Fatalf("plan = %#v, progress=%d list=%d", plan, progressReads, listReads)
	}

	overwrite := BuildPreview(PreviewInput{Overwrite: true, Activities: activities[3:4], Items: items, Progress: func(string) catalog.PlaybackState { return catalog.PlaybackState{Seconds: 2} }, ListAdditions: func(string, bool, map[string]int) int { return 0 }})
	if len(overwrite.Changes) != 1 || overwrite.Summary.Conflicts != 0 {
		t.Fatalf("overwrite = %#v", overwrite)
	}

	many := make([]Activity, 501)
	for index := range many {
		many[index] = Activity{Kind: "movie", Title: "Missing", Year: "2024", Favorite: true}
	}
	truncated := BuildPreview(PreviewInput{Activities: many, Items: items, Progress: func(string) catalog.PlaybackState { return catalog.PlaybackState{} }, ListAdditions: func(string, bool, map[string]int) int { return 0 }})
	if !truncated.Truncated || len(truncated.Items) != 500 {
		t.Fatalf("truncated rows = %d, flag = %t", len(truncated.Items), truncated.Truncated)
	}
}

func TestMergeProgressAndListsPreservesOptimisticState(t *testing.T) { //nolint:cyclop // One test covers the coupled optimistic merge results.
	now := time.Now()
	values := map[string]catalog.PlaybackState{"legacy": {Seconds: 2}, "viewer:conflict": {Seconds: 9}}
	merged, applied, conflicts := MergeProgress(values, "viewer", true, []ProgressChange{
		{TargetID: "conflict", State: catalog.PlaybackState{Seconds: 1}},
		{TargetID: "legacy", Expected: catalog.PlaybackState{Seconds: 2}, State: catalog.PlaybackState{Seconds: 4, Dismissed: true, Session: "old", Revision: 4}},
	}, now)
	if applied != 1 || conflicts != 1 || merged["viewer:legacy"].Seconds != 4 || merged["viewer:legacy"].Dismissed || merged["viewer:legacy"].Session != "" || merged["viewer:legacy"].Updated.IsZero() || len(values) != 2 {
		t.Fatalf("merged=%#v applied=%d conflicts=%d", merged, applied, conflicts)
	}

	lists := map[string]bool{}
	playlists := map[string]map[string]bool{"viewer:Existing": {"same": true}}
	order := map[string][]string{"viewer:Queue": {"first"}}
	smart := map[string]catalog.PlaylistRule{"viewer:Smart": {Sort: "title"}}
	lists, playlists, order, applied = MergeLists(lists, playlists, order, smart, "viewer", []ListChange{
		{TargetID: "two", Favorite: true, Playlists: map[string]int{"Queue": 2, "Smart": 0, "bad/name": 0}},
		{TargetID: "one", Playlists: map[string]int{"Queue": 1}},
		{TargetID: "same", Playlists: map[string]int{"Existing": 0}},
		{TargetID: "equal-one", Playlists: map[string]int{"Equal": 1}},
		{TargetID: "equal-two", Playlists: map[string]int{"Equal": 1}},
	})
	if applied != 5 || !lists["viewer:two"] || !playlists["viewer:Queue"]["one"] || strings.Join(order["viewer:Queue"], ",") != "first,one,two" || strings.Join(order["viewer:Equal"], ",") != "equal-one,equal-two" {
		t.Fatalf("lists=%#v playlists=%#v order=%#v applied=%d", lists, playlists, order, applied)
	}
}

func TestProgressStoragePublishesOnlyPersistedImports(t *testing.T) { //nolint:cyclop // One test proves success, rollback, and conflict side effects.
	values := map[string]catalog.PlaybackState{"viewer:item": {Seconds: 1}}
	var mutex, persistMutex sync.Mutex
	storage := ProgressStorage{Mutex: &mutex, PersistMutex: &persistMutex, Values: &values, File: "progress.json", ProfileID: "viewer", Persist: func(string, any) error { return errors.New("disk") }}
	change := ProgressChange{TargetID: "item", Expected: catalog.PlaybackState{Seconds: 1}, State: catalog.PlaybackState{Seconds: 2}}
	applied, conflicts, err := storage.Import([]ProgressChange{change}, time.Now())
	if err == nil || applied != 0 || conflicts != 0 || values["viewer:item"].Seconds != 1 {
		t.Fatalf("failed import mutated state: values=%#v applied=%d conflicts=%d error=%v", values, applied, conflicts, err)
	}
	persists := 0
	storage.Persist = func(string, any) error { persists++; return nil }
	applied, conflicts, err = storage.Import([]ProgressChange{change}, time.Now())
	if err != nil || applied != 1 || conflicts != 0 || persists != 1 || values["viewer:item"].Seconds != 2 {
		t.Fatalf("successful import = values=%#v applied=%d conflicts=%d persists=%d error=%v", values, applied, conflicts, persists, err)
	}
	_, conflicts, err = storage.Import([]ProgressChange{change}, time.Now())
	if err != nil || conflicts != 1 || persists != 1 || values["viewer:item"].Seconds != 2 {
		t.Fatalf("conflicting import caused side effects: conflicts=%d persists=%d values=%#v error=%v", conflicts, persists, values, err)
	}
}

func TestEngineStopsAtValidationAndMissingProfile(t *testing.T) {
	config := testManagerConfig()
	manager := newTestManager("", config)
	if _, err := manager.Preview(t.Context(), Input{}); err == nil {
		t.Fatal("invalid preview input accepted")
	}
	preview := testPreview()
	preview.ProfileID = "missing"
	manager.previews[preview.ID] = preview
	if _, err := manager.Apply(preview.ID); err == nil || len(manager.previews) != 1 {
		t.Fatalf("missing profile error=%v previews=%#v", err, manager.previews)
	}
}

func TestViewingFormsRejectAmbiguityAndOversize(t *testing.T) { //nolint:cyclop,gocognit,funlen // One boundary test covers every request parser and mutation guard.
	settingsPage, onboardingPage := PreviewPage("/settings/viewing-imports/preview"), PreviewPage("/onboarding/viewing-imports/preview")
	if !settingsPage.AllowSync || settingsPage.ApplyPath != "/settings/viewing-imports/apply" || onboardingPage.AllowSync || onboardingPage.Step != "Step 4 of 4" {
		t.Fatalf("preview pages = %#v / %#v", settingsPage, onboardingPage)
	}
	validID := "ABCDEFGHIJKLMNOP234567"
	request := formRequest("source=plex&url=http%3A%2F%2Fserver&token=t&profileId=p&overwriteExisting=true")
	input, err := ParseImportForm(request)
	if err != nil || input.Source != "plex" || !input.Overwrite {
		t.Fatalf("input = %#v, error = %v", input, err)
	}
	action, err := ParseActionForm(formRequest("id=" + validID))
	if err != nil || action != validID {
		t.Fatalf("action = %q, error = %v", action, err)
	}
	id, interval, err := ParseSyncForm(formRequest("id=" + validID + "&interval=1h"))
	if err != nil || id != validID || interval != "1h" {
		t.Fatalf("sync = %q %q, error = %v", id, interval, err)
	}
	bad := []*http.Request{
		formRequest("id=" + validID + "&id=" + validID),
		formRequest("id=lowercase"),
		formRequest("id=" + validID + "&unknown=x"),
		formRequest("id=" + strings.Repeat("A", 129)),
	}
	bad[0].URL.RawQuery = "x=1"
	for _, request := range bad {
		if _, err := ParseActionForm(request); err == nil {
			t.Fatal("invalid action form was accepted")
		}
	}
	oversized := formRequest("id=" + validID + "&padding=" + strings.Repeat("x", maximumFormBytes))
	if _, err := ParseActionForm(oversized); err == nil {
		t.Fatal("oversized form was accepted")
	}
	wrongType := formRequest("id=" + validID)
	wrongType.Header.Set("Content-Type", "application/json")
	if _, err := ParseActionForm(wrongType); err == nil || ValidID("bad") || ValidID("") {
		t.Fatal("invalid form encoding or identifier was accepted")
	}
	calls := 0
	if _, status, err := ApplyForm(formRequest("id=bad"), func(string) (Summary, error) { calls++; return Summary{}, nil }); err == nil || status != http.StatusBadRequest || calls != 0 {
		t.Fatalf("invalid apply caused side effects: status=%d calls=%d error=%v", status, calls, err)
	}
	if _, status, err := ApplyForm(formRequest("id="+validID), func(string) (Summary, error) { calls++; return Summary{}, errors.New("conflict") }); err == nil || status != http.StatusConflict || calls != 1 {
		t.Fatalf("failed apply = status=%d calls=%d error=%v", status, calls, err)
	}
	if result, status, err := ApplyForm(formRequest("id="+validID), func(string) (Summary, error) { calls++; return Summary{Applied: 1}, nil }); err != nil || status != http.StatusOK || result.Applied != 1 || calls != 2 {
		t.Fatalf("successful apply = result=%#v status=%d calls=%d error=%v", result, status, calls, err)
	}
	previewCalls := 0
	if _, status, err := PreviewForm(context.Background(), wrongType, func(context.Context, Input) (Preview, error) { previewCalls++; return Preview{}, nil }); err == nil || status != http.StatusBadRequest || previewCalls != 0 {
		t.Fatalf("invalid preview caused side effects: status=%d calls=%d error=%v", status, previewCalls, err)
	}
	if _, status, err := PreviewForm(context.Background(), formRequest("source=plex&url=http%3A%2F%2Fserver&token=t&profileId=p"), func(context.Context, Input) (Preview, error) { previewCalls++; return Preview{}, errors.New("source") }); err == nil || status != http.StatusBadRequest || previewCalls != 1 {
		t.Fatalf("failed preview = status=%d calls=%d error=%v", status, previewCalls, err)
	}
	writer := &responseWriterStub{header: make(http.Header)}
	status, err := ServeWebPreview(context.Background(), writer, formRequest("source=plex&url=http%3A%2F%2Fserver&token=t&profileId=p"), func(context.Context, Input) (Preview, error) { return Preview{Source: "plex"}, nil }, func(page WebPreview) error {
		if page.Source != "plex" || !page.AllowSync {
			t.Fatalf("web preview = %#v", page)
		}
		return nil
	})
	if err != nil || status != http.StatusOK || writer.header.Get("Cache-Control") != "no-store" {
		t.Fatalf("served preview = %d %v, headers=%v", status, err, writer.header)
	}
	_, err = ServeWebPreview(context.Background(), writer, formRequest("source=plex"), func(context.Context, Input) (Preview, error) { return Preview{}, nil }, func(WebPreview) error { return errors.New("template details") })
	if !errors.Is(err, ErrPreviewRender) {
		t.Fatalf("renderer error = %v", err)
	}
	for _, id := range []string{strings.Repeat("A", 128), "A", "Z", "2", "7"} {
		if !ValidID(id) {
			t.Fatalf("valid ID rejected: %q", id)
		}
	}
	for _, id := range []string{"@", "[", "1", "8", strings.Repeat("A", 129)} {
		if ValidID(id) {
			t.Fatalf("invalid ID accepted: %q", id)
		}
	}
	if status, err := ServeWebPreview(context.Background(), writer, wrongType, func(context.Context, Input) (Preview, error) { return Preview{}, nil }, func(WebPreview) error { return nil }); err == nil || status != http.StatusBadRequest {
		t.Fatalf("invalid web preview = %d, %v", status, err)
	}
	for _, encoded := range []string{
		"source=plex&source=plex&url=u&token=t&profileId=p",
		"source=plex&url=u&token=t&profileId=p&overwriteExisting=no",
	} {
		if _, err := ParseImportForm(formRequest(encoded)); err == nil {
			t.Fatalf("invalid import form accepted: %q", encoded)
		}
	}
	for _, encoded := range []string{
		"id=" + validID + "&interval=1h&unexpected=true",
		"id=bad&interval=1h",
	} {
		if _, _, err := ParseSyncForm(formRequest(encoded)); err == nil {
			t.Fatalf("invalid sync form accepted: %q", encoded)
		}
	}
}
