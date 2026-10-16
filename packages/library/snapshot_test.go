package library

import (
	"slices"
	"testing"
)

func viewIDs(view *View) []string {
	ids := make([]string, 0, len(view.Items()))
	for _, item := range view.Items() {
		ids = append(ids, item.ID)
	}
	return ids
}

func TestSnapshotViewsNeverShareResultsAcrossPolicies(t *testing.T) {
	t.Parallel()
	items := []Item{
		{ID: "joined", Library: "a\x00b", Rating: "PG"},
		{ID: "spaced", Library: "a b", Rating: "PG"},
		{ID: "a", Library: "a", Rating: "PG"},
		{ID: "b", Library: "b", Rating: "R"},
		{ID: "quoted", Library: `"a"`, Rating: "PG-13"},
	}
	snapshot := NewSnapshot(items, 7)
	policies := []Policy{
		NewPolicy(false, "all", []string{"a\x00b"}),
		NewPolicy(false, "all", []string{"a", "b"}),
		NewPolicy(false, "all", []string{"a b"}),
		NewPolicy(false, "all", []string{`"a"`}),
		NewPolicy(false, "teen", []string{"b", "a", "a"}),
		NewPolicy(false, "family", []string{"all"}),
		NewPolicy(false, "all", []string{"all"}),
		NewPolicy(true, "", nil),
	}
	for range 3 {
		for _, policy := range policies {
			want := make([]string, 0)
			for _, item := range items {
				if policy.Allows(item) {
					want = append(want, item.ID)
				}
			}
			if got := viewIDs(snapshot.View(policy)); !slices.Equal(got, want) {
				t.Fatalf("policy %#v view = %q, want %q", policy, got, want)
			}
		}
	}
	if snapshot.Generation() != 7 || snapshot.Len() != len(items) {
		t.Fatalf("snapshot generation %d length %d", snapshot.Generation(), snapshot.Len())
	}
}
