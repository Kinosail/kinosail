package servertest

import (
	"testing"

	"github.com/MikeO7/kinosail/packages/auditjournal"
	"github.com/MikeO7/kinosail/packages/library"
	"github.com/MikeO7/kinosail/packages/playback"
)

// SecurityClassificationAndMediaLimits verifies secret, bitrate, search, and path policy helpers.
func SecurityClassificationAndMediaLimits(t *testing.T, collectionMatches func(string, []library.Item, string) bool, within func(string, string) bool) { //nolint:cyclop // One shared contract checks the related boundary helpers.
	t.Parallel()
	for key, want := range map[string]bool{"database.password": true, "apiSecret": true, "refreshToken": true, "tls.key": true, "monkey": false, "hostname": false} {
		if got := auditjournal.SecretSetting(key); got != want {
			t.Fatalf("secretSetting(%q) = %v, want %v", key, got, want)
		}
	}
	for name, test := range map[string]struct {
		video, audio string
		maximum      int64
		want         string
	}{
		"unlimited": {"8000k", "192k", 0, "8000k"},
		"capped":    {"8000k", "192k", 4_000_000, "3808k"},
		"unchanged": {"2000k", "192k", 4_000_000, "2000k"},
		"no room":   {"2000k", "192k", 100_000, "2000k"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if got := playback.CappedVideoRate(test.video, test.audio, test.maximum); got != test.want {
				t.Fatalf("cappedVideoRate = %q, want %q", got, test.want)
			}
		})
	}
	items := []library.Item{{Title: "Deep Space"}}
	if !collectionMatches("Favorites", items, "space") || !collectionMatches("Favorites", nil, "favor") || collectionMatches("Favorites", items, "western") {
		t.Fatal("collection search did not consistently match name and member metadata")
	}
	if !within(".", "anything") || !within("media", "media") || !within("media", "media/movies") || within("media", "mediator/movie") {
		t.Fatal("path containment accepted a sibling or rejected a child")
	}
}
