package catalog

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/MikeO7/kinosail/packages/library"
)

func TestBrowseOrderingAndPagingEdges(t *testing.T) { //nolint:cyclop // One compact matrix executes every deterministic ordering fallback.
	t.Parallel()
	items := []*library.Item{{ID: "b", Title: "Beta"}, {ID: "a", Title: "Alpha"}}
	if page := browsePage(items, len(items), len(items), 1); page != nil {
		t.Fatalf("past-end page = %#v", page)
	}
	sortReferences(items, "title", "alpha", "en")
	if items[0].ID != "a" {
		t.Fatalf("ranked items = %#v", items)
	}
	now := time.Now()
	items = []*library.Item{{ID: "b", Added: now}, {ID: "a", Added: now.Add(time.Second)}}
	sortReferences(items, "added", "", "en")
	if items[0].ID != "a" {
		t.Fatalf("added items = %#v", items)
	}
	items = []*library.Item{{ID: "b", Year: "2020"}, {ID: "a", Year: "2021"}}
	sortReferences(items, "year", "", "en")
	if items[0].ID != "a" {
		t.Fatalf("year items = %#v", items)
	}
	items = []*library.Item{{ID: "b", Title: "Same"}, {ID: "a", Title: "Same"}}
	sortReferences(items, "title", "same", "en")
	if items[0].ID != "a" {
		t.Fatalf("query title tie = %#v", items)
	}
	values := []library.Item{{ID: "b", Title: "Same", Added: now, Year: "2020"}, {ID: "a", Title: "Same", Added: now, Year: "2020"}}
	for _, order := range []string{"added", "year", "title"} {
		if got := Sort(append([]library.Item(nil), values...), order); got[0].ID != "a" {
			t.Fatalf("%s tie order = %#v", order, got)
		}
	}
}

func TestSearchRankCoversEveryMatchShape(t *testing.T) {
	t.Parallel()
	tests := []struct {
		title, query string
		want         int
	}{
		{"Alpha", "alpha", 0},
		{"Alphabet", "alpha", 1},
		{"The Alpha", "alpha", 2},
		{"One Alpha", "alpha", 3},
		{"Malphaville", "alpha", 4},
		{"Beta", "alpha", 5},
	}
	for _, test := range tests {
		if got := searchRank(library.Item{Title: test.title}, test.query); got != test.want {
			t.Fatalf("searchRank(%q, %q) = %d; want %d", test.title, test.query, got, test.want)
		}
	}
}

func TestShowAndLetterProjectionEdges(t *testing.T) { //nolint:cyclop // Exact optional-field assertions protect the merged show projection.
	t.Parallel()
	cast := []library.Person{{Name: "Actor"}}
	item := &library.Item{
		Kind: "video", Show: "show", ShowTitle: "Show Title", ShowYear: "2026", ShowPlot: "Plot", ShowGenres: "Drama", ShowStudio: "Studio",
		ShowArtwork: "art", ShowBackdrop: "back", ShowLogo: "logo", ShowCast: cast,
	}
	show := &library.Item{}
	mergeShowReference(show, item)
	if show.Title != item.ShowTitle || show.ShowYear != item.ShowYear || show.ShowPlot != item.ShowPlot || show.ShowGenres != item.ShowGenres || show.ShowStudio != item.ShowStudio || show.ShowArtwork != item.ShowArtwork || show.ShowBackdrop != item.ShowBackdrop || show.ShowLogo != item.ShowLogo || !reflect.DeepEqual(show.ShowCast, cast) {
		t.Fatalf("merged show = %#v", show)
	}
	if got := showReferences([]*library.Item{{Kind: "audio"}, {Kind: "video"}, item}); len(got) != 1 {
		t.Fatalf("show references = %#v", got)
	}
	if titleLetter(" !!! ", "en") != "#" || titleLetter(" 7", "en") != "#" {
		t.Fatal("non-letter title was not grouped under #")
	}
	if lettersOnly("") || lettersOnly("A1") || !lettersOnly("É") || lettersOnly(strings.Repeat("!", 2)) {
		t.Fatal("letter grammar accepted an invalid label")
	}
}
