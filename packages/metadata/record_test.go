package metadata

import (
	"context"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail/packages/library"
)

func TestRecordValidationBoundsEveryPersistedField(t *testing.T) {
	t.Parallel()
	valid := Record{Title: "Title", Year: "2020", Plot: "Plot\nSecond paragraph\tcontinued", Rating: "PG", Tagline: "Tagline", Genres: "Drama", Artwork: "/art", Collection: "Set", ShowTitle: "Show", ShowYear: "2019", ShowPlot: "Plot\r\n\tIndented", ShowArtwork: "/show", ProviderIDs: map[string]string{"tmdb": "1"}, ShowProviderIDs: map[string]string{"tvmaze": "2"}}
	if !boundedMetadata(valid) {
		t.Fatal("valid metadata rejected")
	}
	invalid := []Record{
		{Title: strings.Repeat("x", 201)},
		{Year: "20xx"},
		{Plot: strings.Repeat("x", 5001)},
		{Rating: strings.Repeat("x", 33)},
		{Tagline: strings.Repeat("x", 301)},
		{Genres: strings.Repeat("x", 501)},
		{Collection: strings.Repeat("x", 201)},
		{Artwork: strings.Repeat("x", 4097)},
		{Backdrop: strings.Repeat("x", 4097)},
		{ShowTitle: strings.Repeat("x", 201)},
		{ShowYear: "123"},
		{ShowPlot: strings.Repeat("x", 5001)},
		{ShowArtwork: strings.Repeat("x", 4097)},
		{ShowBackdrop: strings.Repeat("x", 4097)},
		{Title: "hidden\x00text"},
		{Title: "two\nlines"},
		{Plot: "hidden\x00text"},
		{ShowPlot: "hidden\x1ftext"},
		{ProviderIDs: map[string]string{"": "1"}},
		{ProviderIDs: map[string]string{"tmdb": ""}},
		{ProviderIDs: map[string]string{"bad\n": "1"}},
		{ProviderIDs: map[string]string{strings.Repeat("x", 33): "1"}},
		{ProviderIDs: map[string]string{"id": strings.Repeat("x", 129)}},
	}
	tooMany := make(map[string]string)
	for index := range 17 {
		tooMany[string(rune('a'+index))] = "1"
	}
	invalid = append(invalid, Record{ProviderIDs: tooMany})
	for _, record := range invalid {
		if boundedMetadata(record) {
			t.Fatalf("invalid metadata accepted: %#v", record)
		}
	}
}

func TestYearRequiresAtLeastFourCharacters(t *testing.T) {
	t.Parallel()
	if Year("2020-01-02") != "2020" || Year("202") != "" {
		t.Fatal("year normalization mismatch")
	}
}

func TestRecordStateAndFormOperations(t *testing.T) { //nolint:cyclop // The assertions cover one record state boundary.
	t.Parallel()
	base := Record{Artwork: "art", ProviderIDs: map[string]string{"tmdb": "1"}, ShowProviderIDs: map[string]string{"tvmaze": "2"}}
	record, ok := RecordFromForm(url.Values{"title": {" Movie "}, "year": {"2020"}, "plot": {" Plot "}}, base)
	if !ok || record.Title != "Movie" || record.Plot != "Plot" || !record.Owner || record.Artwork != "art" {
		t.Fatalf("form record = %#v, %v", record, ok)
	}
	for _, form := range []url.Values{
		{"title": {"one", "two"}},
		{"title": {strings.Repeat("x", 201)}},
		{"title": {""}},
	} {
		if _, ok := RecordFromForm(form, Record{}); ok {
			t.Fatalf("invalid form accepted: %#v", form)
		}
	}
	clone := Clone(record)
	clone.ProviderIDs["tmdb"] = "changed"
	if record.ProviderIDs["tmdb"] != "1" || !reflect.DeepEqual(clone.ShowProviderIDs, record.ShowProviderIDs) {
		t.Fatal("clone shares provider maps")
	}
	if ValidateRecords(map[string]Record{"movie": record}) != nil {
		t.Fatal("valid persisted state rejected")
	}
	if ValidateRecords(map[string]Record{"": record}) == nil || ValidateRecords(map[string]Record{strings.Repeat("x", 129): record}) == nil || ValidateRecords(map[string]Record{"../movie": record}) == nil {
		t.Fatal("invalid persisted ID accepted")
	}
	tooMany := make(map[string]Record, 100001)
	for index := range 100001 {
		tooMany[strconv.Itoa(index)] = Record{}
	}
	if ValidateRecords(tooMany) == nil {
		t.Fatal("oversized persisted state accepted")
	}
}

func TestMergeRecordsClonesAndOverrides(t *testing.T) {
	t.Parallel()
	current := map[string]Record{"same": {Title: "old", ProviderIDs: map[string]string{"tmdb": "old"}}, "current": {Title: "current"}}
	updates := map[string]Record{"same": {Title: "new", ProviderIDs: map[string]string{"tmdb": "new"}}, "added": {Title: "added"}}
	merged := MergeRecords(current, updates)
	current["current"] = Record{}
	updates["same"].ProviderIDs["tmdb"] = "changed"
	if len(merged) != 3 || merged["same"].Title != "new" || merged["same"].ProviderIDs["tmdb"] != "new" || merged["current"].Title != "current" || merged["added"].Title != "added" {
		t.Fatalf("merged records = %#v", merged)
	}
}

func TestApplyMissingAndResolveTMDBEpisode(t *testing.T) { //nolint:cyclop,staticcheck // The assertions cover one shared episode resolution contract, including an intentional nil context.
	t.Parallel()
	item := library.Item{ID: "episode", Season: 1, Episode: 2, ProviderIDs: map[string]string{"tvdb": "123"}, Year: "local"}
	record := Record{Year: "2020", Plot: "Plot", Artwork: "poster", Backdrop: "landscape", ShowTitle: "Show", ShowYear: "2019", ShowPlot: "Show plot", ShowArtwork: "show-poster", ShowBackdrop: "show-landscape", BackdropChecked: true, ShowProviderIDs: map[string]string{"tmdb": "42"}}
	ApplyMissing(&item, record)
	if item.Year != "local" || item.Plot != "Plot" || item.Backdrop != "landscape" || item.ShowBackdrop != "show-landscape" || item.ShowTitle != "Show" || item.ShowProviderIDs["tmdb"] != "42" {
		t.Fatalf("missing metadata = %#v", item)
	}
	item.ShowProviderIDs["tmdb"] = "changed"
	if record.ShowProviderIDs["tmdb"] != "42" {
		t.Fatal("missing metadata shares provider IDs")
	}
	resolved, err := ResolveTMDBEpisode(t.Context(), item, record, func(_ context.Context, _ library.Item, id int, title, year string, output *Record, poster *string) {
		if id != 42 || title != "Show" || year != "2019" {
			t.Errorf("episode identity = %d %q %q", id, title, year)
		}
		output.Title, output.Year, *poster = "Episode", "2020", "/still.jpg"
	}, func(id string) string { return "/cache/" + id + ".jpg" })
	if err != nil || resolved.Record.Title != "Episode" || resolved.Record.ShowBackdrop != "show-landscape" || !resolved.Record.BackdropChecked || resolved.Record.ProviderIDs["tvdb"] != "123" || len(resolved.Images) != 1 || resolved.Images[0].Target != "/cache/episode.jpg" {
		t.Fatalf("episode result = %#v, %v", resolved, err)
	}
	for name, call := range map[string]func() error{
		"nil context": func() error {
			_, err := ResolveTMDBEpisode(nil, item, record, func(context.Context, library.Item, int, string, string, *Record, *string) {}, func(string) string { return "" }) //nolint:staticcheck // Intentional nil validates fail-closed behavior.
			return err
		},
		"invalid item": func() error {
			_, err := ResolveTMDBEpisode(t.Context(), library.Item{ID: "../episode"}, record, func(context.Context, library.Item, int, string, string, *Record, *string) {}, func(string) string { return "" })
			return err
		},
		"invalid show": func() error {
			_, err := ResolveTMDBEpisode(t.Context(), item, Record{}, func(context.Context, library.Item, int, string, string, *Record, *string) {}, func(string) string { return "" })
			return err
		},
		"invalid provider": func() error {
			_, err := ResolveTMDBEpisode(t.Context(), item, record, func(_ context.Context, _ library.Item, _ int, _, _ string, output *Record, poster *string) {
				output.Title, *poster = "Episode", "/../poster"
			}, func(string) string { return "" })
			return err
		},
	} {
		t.Run(name, func(t *testing.T) {
			if err := call(); err == nil {
				t.Fatal("invalid episode boundary accepted")
			}
		})
	}
}

func TestApplyOwnerMetadata(t *testing.T) { //nolint:cyclop // One mutation fixture verifies all owner-editable metadata fields.
	t.Parallel()
	item := library.Item{Title: "original", Year: "original", Plot: "original", Rating: "original", Tagline: "original", Genres: "original", Artwork: "original", ShowTitle: "original", ShowYear: "original", ShowPlot: "original", ShowArtwork: "original"}
	Apply(&item, Record{Title: "title", Year: "2026", Plot: "plot", Rating: "PG", Tagline: "tagline", Genres: "genre", Artwork: "art", Collection: "collection", ProviderIDs: map[string]string{"tmdb": "1"}, ShowTitle: "show", ShowYear: "2025", ShowPlot: "show plot", ShowArtwork: "show art", ShowProviderIDs: map[string]string{"tmdb": "2"}})
	if item.Title != "title" || item.Year != "2026" || item.Plot != "plot" || item.Rating != "PG" || item.Tagline != "tagline" || item.Genres != "genre" || item.Artwork != "art" || item.Collection != "collection" || item.ProviderIDs["tmdb"] != "1" || item.ShowTitle != "show" || item.ShowYear != "2025" || item.ShowPlot != "show plot" || item.ShowArtwork != "show art" || item.ShowProviderIDs["tmdb"] != "2" {
		t.Fatalf("applied item = %#v", item)
	}
	Apply(&item, Record{})
	if item.Title != "title" || item.Collection != "" {
		t.Fatalf("empty overlay changed populated fields: %#v", item)
	}
}

func TestApplyRecordSetHonorsLocalNFO(t *testing.T) {
	t.Parallel()
	video := filepath.Join(t.TempDir(), "movie.mp4")
	if err := os.WriteFile(strings.TrimSuffix(video, filepath.Ext(video))+".nfo", []byte("local"), 0o600); err != nil {
		t.Fatal(err)
	}
	items := []library.Item{{ID: "missing", Title: "missing"}, {ID: "owner", Title: "old"}, {ID: "local", Title: "local", Path: video}}
	records := map[string]Record{"owner": {Owner: true, Title: "owner", Artwork: "poster"}, "local": {Title: "remote", Plot: "plot", ShowArtwork: "show"}}
	items = ApplyRecords(items, records, func(value string) string { return "current:" + value })
	if items[0].Title != "missing" || items[1].Title != "owner" || items[1].Artwork != "current:poster" || items[2].Title != "local" || items[2].Plot != "plot" || items[2].ShowArtwork != "current:show" {
		t.Fatalf("applied records = %#v", items)
	}
}
