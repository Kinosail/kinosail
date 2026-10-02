package library_test

import (
	"os/exec"
	"slices"
	"testing"

	"github.com/MikeO7/kinosail/packages/library"
)

func TestArchiveListingPreservesWhitespaceNames(t *testing.T) {
	if _, err := exec.LookPath("bsdtar"); err != nil {
		t.Fatalf("bsdtar is required: %v", err)
	}
	for _, format := range []struct {
		name    string
		archive func(*testing.T, []literalArchiveEntry) library.Item
	}{
		{"CBT", writeLiteralArchive},
		{"CB7", writeLiteralCB7Archive},
	} {
		t.Run(format.name, func(t *testing.T) {
			runWhitespaceArchiveMatrix(t, format.archive)
		})
	}
}

func runWhitespaceArchiveMatrix(t *testing.T, factory func(*testing.T, []literalArchiveEntry) library.Item) {
	t.Helper()
	for _, fixture := range []struct {
		name, asset, other string
		images             []string
	}{
		{"leading-space", " leading.png", "other.png", []string{" leading.png", "-dash.png", "other.png", "plain.png"}},
		{"trailing-space", "trailing.png ", "other.png", []string{"-dash.png", "other.png", "plain.png"}},
		{"leading-space-distinct-name", " leading.png", "leading.png", []string{" leading.png", "-dash.png", "leading.png", "plain.png"}},
	} {
		entries := []literalArchiveEntry{
			{fixture.asset, "whitespace named resource\n"},
			{fixture.other, "distinct other resource\n"},
			{"plain.png", "ordinary exact-name control\n"},
			{"-dash.png", "leading-dash exact-name control\n"},
		}
		reversed := slices.Clone(entries)
		slices.Reverse(reversed)
		for _, order := range []struct {
			name    string
			entries []literalArchiveEntry
		}{
			{"first", entries},
			{"reversed", reversed},
		} {
			t.Run(fixture.name+"/"+order.name, func(t *testing.T) {
				item := factory(t, order.entries)
				checkWhitespaceArchiveImages(t, item, fixture.images)
				for _, entry := range order.entries {
					data, err := library.ReadArchiveAsset(t.Context(), item, entry.name)
					if err != nil || string(data) != entry.body {
						t.Errorf("archive asset %q = %q, %v; want exact %q", entry.name, data, err, entry.body)
					}
				}
			})
		}
	}
}

func checkWhitespaceArchiveImages(t *testing.T, item library.Item, want []string) {
	t.Helper()
	var names []string
	for _, image := range library.ArchiveImages(t.Context(), item) {
		names = append(names, image.Name)
	}
	slices.Sort(names)
	if !slices.Equal(names, want) {
		t.Errorf("archive images = %q; want exact member names %q", names, want)
	}
}
