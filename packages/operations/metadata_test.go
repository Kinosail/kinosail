package operations

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/MikeO7/kinosail/packages/library"
	"github.com/MikeO7/kinosail/packages/metadata"
)

func TestRefreshMetadataGroupsEpisodesAndReusesValidRecords(t *testing.T) { //nolint:cyclop,funlen // One refresh proves grouping, reuse, downloads, storage, and index refresh.
	t.Parallel()
	items := []library.Item{
		{ID: "movie", Kind: "video", Title: "Movie"},
		{ID: "episode-1", Kind: "video", Library: "TV", Show: "Example", Title: "First"},
		{ID: "episode-2", Kind: "video", Library: "TV", Show: "Example", Title: "Second"},
		{ID: "episode-3", Kind: "video", Library: "TV", Show: "Example", Title: "Third"},
	}
	records := map[string]metadata.Record{"episode-2": {Title: "Saved second", Plot: "Saved plot"}}
	var resolveCalls, episodeCalls, downloads, refreshes atomic.Int32
	var stored map[string]metadata.Record
	config := metadataFixture(items, records)
	config.Resolve = func(_ context.Context, item library.Item) (metadata.Result, error) {
		resolveCalls.Add(1)
		if item.ID == "episode-1" {
			return metadata.Result{Record: metadata.Record{Title: "First", ShowTitle: "Example", ShowYear: "2026", ShowPlot: "Show plot", ShowArtwork: "provider-show", ShowBackdrop: "provider-backdrop", BackdropChecked: true, ShowProviderIDs: map[string]string{"tmdb": "7"}}}, nil
		}
		return metadata.Result{Record: metadata.Record{Title: "Movie"}}, nil
	}
	config.ResolveEpisode = func(_ context.Context, item library.Item, show metadata.Record) (metadata.Result, error) {
		episodeCalls.Add(1)
		return metadata.Result{Record: metadata.Record{Title: item.Title, ShowTitle: show.ShowTitle, ShowArtwork: "provider-third"}}, nil
	}
	config.Download = func(_ context.Context, result metadata.Result) (metadata.Record, error) {
		downloads.Add(1)
		if result.Record.ShowTitle != "" {
			result.Record.ShowArtwork = "cached-show"
		}
		if result.Record.Title == "Saved second" {
			result.Record.ShowArtwork = "wrong-second-show"
		}
		return result.Record, nil
	}
	config.Store = func(updates map[string]metadata.Record) error { stored = updates; return nil }
	config.RefreshLibrary = func(context.Context) error { refreshes.Add(1); return nil }
	if err := RefreshMetadata(t.Context(), config); err != nil {
		t.Fatal(err)
	}
	if resolveCalls.Load() != 2 || episodeCalls.Load() != 1 || downloads.Load() != 4 || refreshes.Load() != 1 || len(stored) != 4 {
		t.Fatalf("calls resolve=%d episode=%d download=%d refresh=%d stored=%d", resolveCalls.Load(), episodeCalls.Load(), downloads.Load(), refreshes.Load(), len(stored))
	}
	second := stored["episode-2"]
	if second.Title != "Saved second" || second.Plot != "Saved plot" || second.ShowTitle != "Example" || second.ShowYear != "2026" || second.ShowPlot != "Show plot" || second.ShowArtwork != "cached-show" || second.ShowBackdrop != "provider-backdrop" || !second.BackdropChecked || second.ShowProviderIDs["tmdb"] != "7" {
		t.Fatalf("reused episode = %#v", second)
	}
	if stored["episode-1"].ShowArtwork != "cached-show" || stored["episode-3"].Title != "Third" || stored["episode-3"].ShowArtwork != "cached-show" || stored["episode-3"].ShowBackdrop != "provider-backdrop" {
		t.Fatalf("group artwork and resolution = %#v", stored)
	}
}

func TestRefreshMetadataSelectsOnlyEligibleMissingVideo(t *testing.T) { //nolint:funlen // The candidate table protects owner artwork and free provider rules.
	t.Parallel()
	directory := t.TempDir()
	movieArtwork := filepath.Join(directory, "movie.jpg")
	showArtwork := filepath.Join(directory, "show.jpg")
	for _, path := range []string{movieArtwork, showArtwork} {
		if err := os.WriteFile(path, []byte("image"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	items := []library.Item{
		{ID: "photo", Kind: "photo"},
		{ID: "owner", Kind: "video"},
		{ID: "cached-movie", Kind: "video", Artwork: movieArtwork},
		{ID: "cached-show", Kind: "video", Show: "Cached", ShowArtwork: showArtwork, Season: 1, Episode: 1, ProviderIDs: map[string]string{"tvdb": "1"}},
		{ID: "unconfigured", Kind: "video"},
		{ID: "eligible", Kind: "video", Show: "Example", Season: 0, Episode: 2, ProviderIDs: map[string]string{"tvdb": "42"}},
	}
	records := map[string]metadata.Record{
		"owner":        {Title: "Owner", Owner: true},
		"cached-movie": {Title: "Movie"},
		"cached-show":  {Title: "Episode"},
	}
	config := metadataFixture(items, records)
	config.Configured = false
	var resolved []string
	config.Resolve = func(_ context.Context, item library.Item) (metadata.Result, error) {
		resolved = append(resolved, item.ID)
		return metadata.Result{Record: metadata.Record{Title: item.ID}}, nil
	}
	var stored map[string]metadata.Record
	config.Store = func(updates map[string]metadata.Record) error { stored = updates; return nil }
	if err := RefreshMetadata(t.Context(), config); err != nil {
		t.Fatal(err)
	}
	if strings.Join(resolved, ",") != "eligible" || len(stored) != 1 || stored["eligible"].Title != "eligible" {
		t.Fatalf("resolved=%v stored=%#v", resolved, stored)
	}
}

func TestRefreshMetadataPreservesPartialSuccessAndFailureOrder(t *testing.T) { //nolint:funlen // One table proves each ordered persistence failure boundary.
	t.Parallel()
	failure := errors.New("failure")
	item := library.Item{ID: "movie", Kind: "video"}
	for _, test := range []struct {
		name      string
		configure func(*MetadataRefresh)
		want      string
		stored    int
		refreshed int
	}{
		{"provider unavailable", func(config *MetadataRefresh) { config.Available = false }, "metadata provider is not configured", 0, 0},
		{"resolve", func(config *MetadataRefresh) {
			config.Resolve = func(context.Context, library.Item) (metadata.Result, error) { return metadata.Result{}, failure }
		}, "metadata refresh failed for 1 of 1 items", 0, 0},
		{"download", func(config *MetadataRefresh) {
			config.Download = func(_ context.Context, result metadata.Result) (metadata.Record, error) {
				return result.Record, failure
			}
		}, "metadata refresh failed for 1 of 1 items", 1, 1},
		{"store", func(config *MetadataRefresh) {
			config.Store = func(map[string]metadata.Record) error { return failure }
		}, "failure", 1, 0},
		{"library", func(config *MetadataRefresh) { config.RefreshLibrary = func(context.Context) error { return failure } }, "failure", 1, 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			var stored, refreshed int
			config := metadataFixture([]library.Item{item}, nil)
			test.configure(&config)
			baseStore := config.Store
			config.Store = func(updates map[string]metadata.Record) error { stored++; return baseStore(updates) }
			baseRefresh := config.RefreshLibrary
			config.RefreshLibrary = func(ctx context.Context) error { refreshed++; return baseRefresh(ctx) }
			err := RefreshMetadata(t.Context(), config)
			if err == nil || err.Error() != test.want || stored != test.stored || refreshed != test.refreshed {
				t.Fatalf("error=%v stored=%d refreshed=%d", err, stored, refreshed)
			}
		})
	}
}

func TestRefreshMetadataReturnsNoWorkWithoutSideEffects(t *testing.T) {
	t.Parallel()
	config := metadataFixture([]library.Item{{ID: "audio", Kind: "audio"}}, nil)
	config.Store = func(map[string]metadata.Record) error { t.Fatal("stored empty refresh"); return nil }
	config.RefreshLibrary = func(context.Context) error { t.Fatal("refreshed empty metadata"); return nil }
	if err := RefreshMetadata(t.Context(), config); err != nil {
		t.Fatal(err)
	}
}

func TestRefreshMetadataRejectsIncompleteDependenciesWithoutSideEffects(t *testing.T) {
	t.Parallel()
	base := metadataFixture([]library.Item{{ID: "movie", Kind: "video"}}, nil)
	invalid := []MetadataRefresh{
		base,
		base,
		base,
		base,
		base,
		base,
	}
	invalid[0].Record = nil
	invalid[1].Resolve = nil
	invalid[2].ResolveEpisode = nil
	invalid[3].Download = nil
	invalid[4].Store = nil
	invalid[5].RefreshLibrary = nil
	for position, config := range invalid {
		if err := RefreshMetadata(t.Context(), config); err == nil || err.Error() != "metadata refresh dependencies are incomplete" {
			t.Fatalf("dependency %d error = %v", position, err)
		}
	}
	if err := RefreshMetadata(nil, base); err == nil || err.Error() != "metadata refresh dependencies are incomplete" { //nolint:staticcheck // The public seam must reject a nil context.
		t.Fatalf("nil context error = %v", err)
	}
}

func TestRefreshMetadataResolvesMissingEpisodeAndReportsGroupFailure(t *testing.T) {
	t.Parallel()
	items := []library.Item{{ID: "first", Kind: "video", Library: "TV", Show: "Show"}, {ID: "second", Kind: "video", Library: "TV", Show: "Show"}}
	config := metadataFixture(items, map[string]metadata.Record{"second": {Plot: "invalid without title"}})
	config.ResolveEpisode = func(_ context.Context, item library.Item, show metadata.Record) (metadata.Result, error) {
		if item.ID != "second" || show.Title != "first" {
			t.Fatalf("episode input = %#v %#v", item, show)
		}
		return metadata.Result{}, errors.New("episode failed")
	}
	var stored bool
	config.Store = func(map[string]metadata.Record) error { stored = true; return nil }
	err := RefreshMetadata(t.Context(), config)
	if err == nil || err.Error() != "metadata refresh failed for 2 of 2 items" || stored {
		t.Fatalf("error=%v stored=%t", err, stored)
	}
}

func TestRefreshMetadataCountsFailuresAcrossIndependentGroups(t *testing.T) {
	t.Parallel()
	items := []library.Item{{ID: "one", Kind: "video"}, {ID: "two", Kind: "video"}}
	config := metadataFixture(items, nil)
	config.Resolve = func(context.Context, library.Item) (metadata.Result, error) {
		return metadata.Result{}, errors.New("failed")
	}
	err := RefreshMetadata(t.Context(), config)
	if err == nil || err.Error() != "metadata refresh failed for 2 of 2 items" {
		t.Fatalf("error = %v", err)
	}
}

func TestRefreshMetadataDiscardsCompletedWorkAfterCancellation(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(t.Context())
	config := metadataFixture([]library.Item{{ID: "movie", Kind: "video"}}, nil)
	config.Resolve = func(context.Context, library.Item) (metadata.Result, error) {
		cancel()
		return metadata.Result{Record: metadata.Record{Title: "resolved"}}, nil
	}
	config.Store = func(map[string]metadata.Record) error { t.Fatal("stored canceled work"); return nil }
	if err := RefreshMetadata(ctx, config); !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v", err)
	}
}

func metadataFixture(items []library.Item, records map[string]metadata.Record) MetadataRefresh {
	var mu sync.Mutex
	return MetadataRefresh{
		Available: true, Configured: true, Items: items,
		Record: func(id string) (metadata.Record, bool) {
			mu.Lock()
			defer mu.Unlock()
			record, found := records[id]
			return record, found
		},
		Resolve: func(_ context.Context, item library.Item) (metadata.Result, error) {
			return metadata.Result{Record: metadata.Record{Title: item.ID}}, nil
		},
		ResolveEpisode: func(_ context.Context, item library.Item, show metadata.Record) (metadata.Result, error) {
			return metadata.Result{Record: metadata.Record{Title: item.ID, ShowTitle: show.ShowTitle}}, nil
		},
		Download:       func(_ context.Context, result metadata.Result) (metadata.Record, error) { return result.Record, nil },
		Store:          func(map[string]metadata.Record) error { return nil },
		RefreshLibrary: func(context.Context) error { return nil },
	}
}

func TestMissingMetadataBackfillsCastOnceAndPreservesOwner(t *testing.T) {
	path := filepath.Join(t.TempDir(), "poster.jpg")
	if err := os.WriteFile(path, []byte("poster"), 0o600); err != nil {
		t.Fatal(err)
	}
	items := []library.Item{{ID: "old", Kind: "video", Artwork: path}, {ID: "complete", Kind: "video", Artwork: path}, {ID: "owner", Kind: "video", Artwork: path}}
	config := metadataFixture(items, map[string]metadata.Record{"old": {Title: "Old", CastFetched: true}, "complete": {Title: "Complete", CastFetched: true, BackdropChecked: true}, "owner": {Title: "Owner", Owner: true}})
	config.Configured = true
	missing := missingMetadata(config)
	if len(missing) != 1 || missing[0].ID != "old" {
		t.Fatalf("backdrop backfill = %#v", missing)
	}
}

func TestMissingBackdropFileIsRetried(t *testing.T) {
	item := library.Item{ID: "movie", Kind: "video"}
	config := metadataFixture([]library.Item{item}, map[string]metadata.Record{"movie": {Title: "Movie", CastFetched: true, BackdropChecked: true, Backdrop: filepath.Join(t.TempDir(), "missing.jpg")}})
	if missing := missingMetadata(config); len(missing) != 1 || missing[0].ID != item.ID {
		t.Fatalf("missing backdrop was not retried: %#v", missing)
	}
}

func TestShowBackdropDownloadFailureKeepsEveryEpisodeRetryable(t *testing.T) {
	items := []library.Item{{ID: "first", Kind: "video", Library: "TV", Show: "Series"}, {ID: "second", Kind: "video", Library: "TV", Show: "Series"}}
	config := metadataFixture(items, nil)
	config.Resolve = func(context.Context, library.Item) (metadata.Result, error) {
		return metadata.Result{Record: metadata.Record{Title: "First", ShowTitle: "Series", ShowBackdrop: "pending.jpg", BackdropChecked: true}}, nil
	}
	config.ResolveEpisode = func(context.Context, library.Item, metadata.Record) (metadata.Result, error) {
		return metadata.Result{Record: metadata.Record{Title: "Second", ShowTitle: "Series", ShowBackdrop: "pending.jpg", BackdropChecked: true}}, nil
	}
	config.Download = func(_ context.Context, result metadata.Result) (metadata.Record, error) {
		if result.Record.Title == "First" {
			result.Record.ShowBackdrop, result.Record.BackdropChecked = "", false
			return result.Record, errors.New("backdrop unavailable")
		}
		return result.Record, nil
	}
	var stored map[string]metadata.Record
	config.Store = func(updates map[string]metadata.Record) error { stored = updates; return nil }
	if err := RefreshMetadata(t.Context(), config); err == nil || len(stored) != 2 || stored["first"].BackdropChecked || stored["second"].BackdropChecked || stored["second"].ShowBackdrop != "" {
		t.Fatalf("failed shared backdrop = %#v, error = %v", stored, err)
	}
}
