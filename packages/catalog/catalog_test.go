package catalog

import (
	"context"
	"errors"
	"math"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/MikeO7/kinosail/packages/library"
)

func TestBrowseValidatesBeforeSelectionAndBuildsStablePages(t *testing.T) { //nolint:cyclop // One test covers validation, selection, and page boundaries.
	bad := []url.Values{
		{"unknown": {"x"}},
		{"q": {"one", "two"}},
		{"view": {"bad"}},
		{"sort": {"bad"}},
		{"letter": {""}},
		{"letter": {"A"}, "q": {"a"}},
		{"offset": {"-1"}},
		{"limit": {"0"}},
		{"q": {strings.Repeat("x", 513)}},
		{"limit": {"12345678901"}},
	}
	for _, values := range bad {
		if _, err := ParseBrowse(values, "en"); !errors.Is(err, ErrInvalidBrowse) {
			t.Fatalf("ParseBrowse(%v) error = %v", values, err)
		}
	}

	now := time.Now()
	items := []library.Item{
		{ID: "b", Kind: "video", Title: "The Beacon", Year: "2020", Added: now.Add(-time.Hour)},
		{ID: "a", Kind: "video", Title: "Arrival", Year: "2016", Added: now, Cast: []library.Person{{Name: "Amy Adams"}}},
		{ID: "c", Kind: "book", Title: "Élan"},
	}
	candidates := []Candidate{{Item: &items[0], Watched: true, Updated: now}, {Item: &items[1], Listed: true, Updated: now.Add(time.Hour)}, {Item: &items[2]}}
	browse, err := ParseBrowse(url.Values{"q": {"amy"}, "limit": {"1"}}, "en")
	if err != nil {
		t.Fatal(err)
	}
	result, err := browse.Apply(candidates)
	if err != nil || result.Total != 1 || len(result.Items) != 1 || result.Items[0].ID != "a" || result.View != "all" || result.Sort != "title" {
		t.Fatalf("result = %#v, error = %v", result, err)
	}
	all := result.AllItems()
	all[0].Title = "changed"
	if result.AllItems()[0].Title == "changed" {
		t.Fatal("AllItems exposed result storage")
	}

	history, _ := ParseBrowse(url.Values{"view": {"history"}, "limit": {"1"}}, "en")
	result, err = history.Apply(candidates)
	if err != nil || result.Items[0].ID != "a" || result.NextURL() == "" || result.PreviousURL() != "" {
		t.Fatalf("history = %#v, error = %v", result, err)
	}
}

func TestBrowseLibraryValidatesBeforeLoading(t *testing.T) {
	loads := 0
	load := func() ([]*library.Item, error) { loads++; return nil, errors.New("index") }
	if _, err := BrowseLibrary(url.Values{"unknown": {"x"}}, "en", load, func() BrowseAccess { return BrowseAccess{} }); !errors.Is(err, ErrInvalidBrowse) || loads != 0 {
		t.Fatalf("invalid query loaded index: loads=%d error=%v", loads, err)
	}
	if _, err := BrowseLibrary(nil, "en", load, func() BrowseAccess { return BrowseAccess{} }); err == nil || loads != 1 {
		t.Fatalf("index error = %v, loads=%d", err, loads)
	}
	item := library.Item{ID: "visible", Title: "Visible"}
	var progressMutex sync.Mutex
	var listMutex sync.RWMutex
	progress := map[string]PlaybackState{"viewer:visible": {Watched: true}}
	listed := map[string]bool{"viewer:visible": true}
	result, err := BrowseLibrary(url.Values{"view": {"list"}}, "en", func() ([]*library.Item, error) { return []*library.Item{&item}, nil }, func() BrowseAccess {
		return BrowseAccess{ProgressMutex: &progressMutex, ListMutex: &listMutex, Progress: &progress, Listed: &listed, ProfileID: "viewer", Visible: func(library.Item) bool { return true }}
	})
	if err != nil || result.Total != 1 || result.Items[0].ID != item.ID || DefaultPageSize != 100 {
		t.Fatalf("result=%#v error=%v", result, err)
	}
}

func TestSmallBrowseHelpers(t *testing.T) {
	progress := map[string]PlaybackState{"viewer:item": {Seconds: 2}, "legacy": {Seconds: 1}}
	var mutex sync.Mutex
	if got := ProfileProgress(progress, "viewer", false, "item"); got.Seconds != 2 {
		t.Fatalf("profile progress = %#v", got)
	}
	if got := ProfileProgress(progress, "viewer", true, "legacy"); got.Seconds != 1 {
		t.Fatalf("owner progress = %#v", got)
	}
	if value, err := SingleValue(url.Values{"q": {" value "}}, "q", 10); err != nil || value != " value " {
		t.Fatalf("value=%q error=%v", value, err)
	}
	if _, err := SingleValue(url.Values{"q": {"a", "b"}}, "q", 10); err == nil {
		t.Fatal("repeated query was accepted")
	}
	access := NewBrowseAccess(&mutex, &sync.RWMutex{}, &progress, &map[string]bool{}, "viewer", false, func(library.Item) bool { return true })
	if access.ProfileID != "viewer" || access.Owner || access.Visible == nil {
		t.Fatalf("access=%#v", access)
	}
}

func TestBrowseViewsSortsLettersAndShows(t *testing.T) { //nolint:cyclop,gocognit // One table covers the complete public browse vocabulary.
	now := time.Now()
	items := []library.Item{
		{ID: "movie", Kind: "video", Title: "10 Things", Year: "1999", Added: now},
		{ID: "e2", Kind: "video", Title: "Episode 2", Show: "show", ShowTitle: "Éclair", ShowYear: "2024", Season: 1, Episode: 2},
		{ID: "e1", Kind: "video", Title: "Episode 1", Show: "show", ShowTitle: "Éclair", ShowPlot: "Plot", ShowGenres: "Drama", ShowStudio: "Studio", ShowArtwork: "art", ShowBackdrop: "back", ShowLogo: "logo", ShowCast: []library.Person{{Name: "Actor"}}, Season: 1, Episode: 1},
		{ID: "audio", Kind: "audio", Title: "Song"},
		{ID: "audiobook", Kind: "audiobook", Title: "Book audio"},
		{ID: "book", Kind: "book", Title: "Book"},
		{ID: "photo", Kind: "photo", Title: "Photo"},
	}
	candidates := make([]Candidate, len(items))
	for index := range items {
		candidates[index] = Candidate{Item: &items[index], Listed: items[index].ID == "movie", Watched: items[index].ID != "movie"}
	}
	expected := map[string]int{"list": 1, "unwatched": 1, "movies": 1, "shows": 1, "music": 1, "audiobooks": 1, "books": 1, "photos": 1, "collections": 7, "playlists": 7}
	for view, total := range expected {
		browse, err := ParseBrowse(url.Values{"view": {view}}, "fr")
		if err != nil {
			t.Fatal(err)
		}
		result, err := browse.Apply(candidates)
		if err != nil || result.Total != total {
			t.Errorf("%s total = %d, error = %v", view, result.Total, err)
		}
	}

	for _, order := range []string{"added", "year", "title"} {
		browse, _ := ParseBrowse(url.Values{"sort": {order}}, "en")
		if _, err := browse.Apply(candidates); err != nil {
			t.Errorf("sort %s: %v", order, err)
		}
	}

	letterBrowse, _ := ParseBrowse(url.Values{"letter": {"e"}, "limit": {"1"}}, "fr")
	letterResult, err := letterBrowse.Apply(candidates)
	current := false
	for _, letter := range letterResult.Letters {
		current = current || letter.Label == "E" && letter.Current
	}
	if err != nil || letterResult.Letter != "E" || len(letterResult.Items) != 1 || !current {
		t.Fatalf("letter result = %#v, error = %v", letterResult, err)
	}
	badOffset, _ := ParseBrowse(url.Values{"letter": {"E"}, "offset": {"0"}, "limit": {"1"}}, "fr")
	if _, err := badOffset.Apply(candidates); !errors.Is(err, ErrInvalidBrowse) {
		t.Fatalf("bad letter offset error = %v", err)
	}
}

func TestSearchListAndProgressRules(t *testing.T) { //nolint:cyclop,gocognit // One table exercises the shared catalog rule surface.
	items := []library.Item{{ID: "b", Title: "B"}, {ID: "a", Title: "A", SortTitle: "Z", Plot: "Café", ShowCast: []library.Person{{Name: "Zoë"}}}}
	if len(Filter(items, "cafe")) != 1 || !Matches(items[1], "zoe") || len(Filter(items, "")) != 2 {
		t.Fatal("normalized search did not match metadata")
	}
	if Sort(append([]library.Item(nil), items...), "title")[0].ID != "b" || Sort(append([]library.Item(nil), items...), "year")[0].ID != "a" {
		t.Fatal("sort did not use canonical tie breaks")
	}

	state := CloneListState(nil, map[string]map[string]bool{"viewer:Queue": {"a": true}}, map[string][]string{"viewer:Queue": {"a"}}, nil)
	if err := ValidateListState(state); err != nil {
		t.Fatal(err)
	}
	if got := ListAdditions(state, "viewer", "b", true, map[string]int{"Queue": 1, "bad/name": 2}); got != 2 {
		t.Fatalf("ListAdditions = %d", got)
	}
	invalid := []ListState{
		{Values: map[string]bool{"": true}},
		{Playlists: map[string]map[string]bool{"": {}}},
		{PlaylistOrder: map[string][]string{"v:q": {"a", "a"}}},
		{Smart: map[string]PlaylistRule{"v:q": {Kind: "invalid", Sort: "title"}}},
	}
	for _, candidate := range invalid {
		if ValidateListState(CloneListState(candidate.Values, candidate.Playlists, candidate.PlaylistOrder, candidate.Smart)) == nil {
			t.Fatalf("invalid list state accepted: %#v", candidate)
		}
	}

	progress := map[string]PlaybackState{"legacy": {Seconds: 1}}
	previous, current, changed, err := progressChange(progress, "viewer:item", func(state PlaybackState) (PlaybackState, bool, error) {
		state.Seconds = 42
		return state, true, nil
	})
	if err != nil || previous.Seconds != 0 || current.Seconds != 42 || !changed || len(progress) != 1 {
		t.Fatalf("progress change = %#v %#v %t %v", previous, current, changed, err)
	}
	if _, _, _, err := progressChange(progress, "", func(PlaybackState) (PlaybackState, bool, error) { return PlaybackState{}, true, nil }); !errors.Is(err, ErrInvalidProgressState) {
		t.Fatalf("invalid key error = %v", err)
	}
	if _, _, changed, err := progressChange(progress, "x", func(state PlaybackState) (PlaybackState, bool, error) { return state, false, errors.New("stop") }); changed || err == nil {
		t.Fatal("rejected progress change was not preserved")
	}
	if ValidateStoredProgress(map[string]PlaybackState{"x": {Seconds: math.NaN()}}) == nil || !ValidPlaybackState(PlaybackState{Seconds: 1, ReaderPage: 1, Session: "s"}) {
		t.Fatal("progress validation did not enforce finite bounds")
	}
}

func TestStatePersistenceAndShowSelection(t *testing.T) { //nolint:cyclop // One transaction test covers success, rollback, and show selection.
	values := map[string]PlaybackState{}
	var mutex, persistMutex sync.Mutex
	persisted := false
	storage := ProgressStorage{Mutex: &mutex, PersistMutex: &persistMutex, Values: &values, File: "progress.json", Persist: func(string, any) error { persisted = true; return nil }}
	if _, _, changed, err := storage.Update("viewer:item", func(state PlaybackState) (PlaybackState, bool, error) { state.Seconds = 3; return state, true, nil }); err != nil || !changed || !persisted || values["viewer:item"].Seconds != 3 {
		t.Fatalf("stored values = %#v, changed = %t, error = %v", values, changed, err)
	}
	storage.Persist = func(string, any) error { return errors.New("disk") }
	if _, _, changed, err := storage.Update("viewer:item", func(state PlaybackState) (PlaybackState, bool, error) { state.Seconds = 9; return state, true, nil }); err == nil || changed || values["viewer:item"].Seconds != 3 {
		t.Fatal("failed persistence changed memory")
	}

	writes := 0
	state := CloneListState(map[string]bool{"v:i": true}, nil, nil, nil)
	if err := CommitListState(context.Background(), nil, ListPaths{Values: "lists.json"}, func(file string, value any) error { writes++; return nil }, state, 1); err != nil || writes != 1 {
		t.Fatalf("list commit writes = %d, error = %v", writes, err)
	}
	if len(ListDocuments(state, 15)) != 4 {
		t.Fatal("selected list documents were incomplete")
	}

	show := library.Show{Episodes: []library.Item{{ID: "one", Title: "One"}, {ID: "two", Title: "Two"}}}
	action := ShowPlay(show, func(id string) PlaybackState { return PlaybackState{Watched: id == "one", Seconds: 7} })
	if action.ID != "two" || action.Label != "Resume" {
		t.Fatalf("action = %#v", action)
	}
	if ShowPlay(library.Show{}, func(string) PlaybackState { return PlaybackState{} }) != nil {
		t.Fatal("empty Show returned an action")
	}
	action = ShowPlay(show, func(string) PlaybackState { return PlaybackState{Watched: true} })
	if action.ID != "one" || action.Label != "Play again" {
		t.Fatalf("replay action = %#v", action)
	}
}
