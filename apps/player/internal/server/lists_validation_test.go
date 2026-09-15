package server

import (
	"fmt"
	"strings"
	"testing"
)

func TestPlaylistCreationBoundsItemsBeforeSideEffects(t *testing.T) {
	many := make([]string, 201)
	for index := range many {
		many[index] = fmt.Sprintf("id-%d", index)
	}
	for name, ids := range map[string][]string{
		"too many":     many,
		"oversized ID": {strings.Repeat("x", 257)},
	} {
		t.Run(name, func(t *testing.T) {
			store := newListStore("")
			if err := store.Create(t.Context(), "viewer", "Rejected", ids...); err == nil {
				t.Fatal("invalid playlist was accepted")
			}
			if len(store.playlists) != 0 || len(store.playlistOrder) != 0 {
				t.Fatalf("rejected playlist changed state: %#v %#v", store.playlists, store.playlistOrder)
			}
		})
	}
}
