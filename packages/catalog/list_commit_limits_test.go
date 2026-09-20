package catalog

import (
	"fmt"
	"testing"
)

func TestListCommitRejectsStateThatCannotBeRestored(t *testing.T) {
	for _, kind := range []string{"playlists", "smart", "order", "members"} {
		t.Run(kind, func(t *testing.T) {
			state := CloneListState(nil, nil, nil, nil)
			for index := range 10_001 {
				key := fmt.Sprintf("viewer:%d", index)
				switch kind {
				case "playlists":
					state.Playlists[key] = map[string]bool{}
				case "smart":
					state.Smart[key] = PlaylistRule{Sort: "title"}
				case "order":
					state.PlaylistOrder[key] = []string{}
				case "members":
					if state.Playlists["viewer:list"] == nil {
						state.Playlists["viewer:list"] = map[string]bool{}
					}
					state.Playlists["viewer:list"][key] = true
				}
			}
			writes := 0
			err := CommitListState(t.Context(), nil, ListPaths{"values", "playlists", "order", "smart"}, func(string, any) error { writes++; return nil }, state, 15)
			if err == nil || writes != 0 {
				t.Fatalf("invalid state was persisted: %v, writes=%d", err, writes)
			}
		})
	}
}
