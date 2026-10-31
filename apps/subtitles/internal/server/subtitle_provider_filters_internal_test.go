package server

import (
	"testing"

	"github.com/MikeO7/kinosail/packages/library"
)

func TestSubDLRejectsForcedOnlyReleases(t *testing.T) {
	t.Parallel()
	item := library.Item{Kind: "video", Title: "Arrival", Path: "/media/Arrival.2016.mkv"}
	for release, valid := range map[string]bool{
		"Arrival.2016.1080p.BluRay.FORCED": false,
		"Arrival 2016 forced subs":         false,
		"Arrival.2016.1080p.BluRay-GROUP":  true,
		"Arrival.2016.Unforced.Cut.BluRay": true,
	} {
		candidate := subtitleCandidate{URL: "/subtitle/arrival.zip", Release: release, Language: "EN"}
		if got := validSubtitleCandidate(item, "en", candidate); got != valid {
			t.Errorf("release %q valid = %t, want %t", release, got, valid)
		}
	}
}
