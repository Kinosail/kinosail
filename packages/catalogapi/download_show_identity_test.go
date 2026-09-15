package catalogapi

import (
	"testing"

	"github.com/MikeO7/kinosail/packages/catalog"
	"github.com/MikeO7/kinosail/packages/library"
)

func TestEpisodeDownloadIdentityMatchesOrganizedShow(t *testing.T) {
	items := []library.Item{
		{ID: "one", Kind: "video", Show: "Example", ShowTitle: "Display title", Season: 1, Episode: 1, Size: 123},
		{ID: "two", Kind: "video", Show: "EXAMPLE", Season: 2, Episode: 1},
	}
	_, shows := library.Organize(items)
	if len(shows) != 1 {
		t.Fatalf("shows = %v", shows)
	}
	for _, item := range items {
		result := ProjectItem(item, catalog.PlaybackState{}, ItemAccess{})
		if result.ShowID != shows[0].ID || result.Size != item.Size || result.Download != "" {
			t.Fatalf("episode projection = %#v", result)
		}
	}
	for _, item := range []library.Item{{Kind: "video"}, {Kind: "audio", Show: "Example"}} {
		if result := ProjectItem(item, catalog.PlaybackState{}, ItemAccess{}); result.ShowID != "" {
			t.Fatalf("non-episode has show ID: %#v", result)
		}
	}
}
