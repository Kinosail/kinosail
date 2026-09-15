package metadata

import (
	"context"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail/packages/library"
)

func TestResolveTMDBEpisodeRejectsOversizedArtworkTarget(t *testing.T) {
	t.Parallel()
	item := library.Item{ID: "episode", Season: 1, Episode: 1}
	show := Record{ShowTitle: "Show", ShowProviderIDs: map[string]string{"tmdb": "1"}}
	_, err := ResolveTMDBEpisode(t.Context(), item, show, func(_ context.Context, _ library.Item, _ int, _, _ string, record *Record, poster *string) {
		record.Title = "Episode"
		*poster = "/still.jpg"
	}, func(string) string { return strings.Repeat("x", 4097) })
	if err == nil || err.Error() != "metadata artwork path is invalid" {
		t.Fatalf("oversized artwork error = %v", err)
	}
}

func TestTVDBProviderIDReturnsIndependentOptionalIdentity(t *testing.T) {
	t.Parallel()
	if got := TVDBProviderID(library.Item{}); got != nil {
		t.Fatalf("missing TVDB identity = %#v", got)
	}
	item := library.Item{ProviderIDs: map[string]string{"tvdb": " 123 "}}
	got := TVDBProviderID(item)
	got["tvdb"] = "changed"
	if item.ProviderIDs["tvdb"] != " 123 " {
		t.Fatal("TVDB identity result aliases the item map")
	}
}
