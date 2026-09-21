package playerweb

import (
	"testing"

	"github.com/MikeO7/kinosail/packages/library"
)

func TestNewPlayerDataStartsOnTheOriginalMediaTimeline(t *testing.T) {
	data := NewPlayerData(library.Item{ID: "movie", Title: "Movie"}, "viewer", "playback-session")
	if data.Source != "/media/movie" || data.HLS || data.Start != 0 || data.PlaybackToken != "" {
		t.Fatalf("new player starts transformed playback: %#v", data)
	}
	if data.ModeURL != "?compatible=1" || data.ModeLabel != "Compatibility stream" {
		t.Fatal("compatible playback action missing")
	}
	if data.ViewerProfile != "viewer" || data.PlaybackSession != "playback-session" || data.Title != "Movie" {
		t.Fatal("player identity or media metadata lost")
	}
}
