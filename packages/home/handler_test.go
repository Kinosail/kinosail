package home

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/MikeO7/kinosail/packages/catalog"
	"github.com/MikeO7/kinosail/packages/library"
	"github.com/MikeO7/kinosail/packages/navigation"
)

type testSource struct {
	result        catalog.Result
	err           error
	shell         Shell
	cards         CatalogProjection[string, string, string]
	personal      PersonalProjection[string]
	artwork       map[string]bool
	personalCalls int
}

func (source *testSource) Browse(*http.Request) (catalog.Result, error) {
	return source.result, source.err
}

func (source *testSource) Shell(*http.Request, string) Shell { return source.shell }

func (source *testSource) Catalog(*http.Request, []library.Show, catalog.Result, bool) CatalogProjection[string, string, string] {
	return source.cards
}

func (source *testSource) Personal(*http.Request, []library.Item) PersonalProjection[string] {
	source.personalCalls++
	return source.personal
}

func (source *testSource) HasArtwork(item library.Item) bool { return source.artwork[item.ID] }

type testView struct {
	data          any
	fullCalls     int
	templateCalls int
	err           error
}

func (view *testView) Execute(_ http.ResponseWriter, _ *http.Request, data any) error {
	view.fullCalls++
	view.data = data
	return view.err
}

func (view *testView) ExecuteTemplate(_ http.ResponseWriter, _ *http.Request, name string, data any) error {
	if name != "libraryResults" {
		panic("unexpected template")
	}
	view.templateCalls++
	view.data = data
	return view.err
}

type failureRecord struct {
	message string
	status  int
}

func browseResult(t *testing.T, values url.Values, items []library.Item) catalog.Result {
	t.Helper()
	browse, err := catalog.ParseBrowse(values, "en")
	if err != nil {
		t.Fatal(err)
	}
	candidates := make([]catalog.Candidate, len(items))
	for index := range items {
		candidates[index].Item = &items[index]
	}
	result, err := browse.Apply(candidates)
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func TestHandlerBuildsCompleteLandingPage(t *testing.T) { //nolint:cyclop // The landing-page contract intentionally checks every populated projection.
	now := time.Now()
	items := []library.Item{
		{ID: "movie", Kind: "video", Title: "Movie", Added: now},
		{ID: "episode", Kind: "video", Title: "Pilot", Show: "series", ShowTitle: "Series", ShowArtwork: "poster", Added: now.Add(-time.Minute)},
		{ID: "song", Kind: "audio", Title: "Song", Added: now.Add(-2 * time.Minute)},
		{ID: "track", Kind: "audio", Title: "Track", Album: "Record", Artist: "Artist", Added: now.Add(-3 * time.Minute)},
		{ID: "spoken", Kind: "audiobook", Title: "Spoken", Added: now.Add(-4 * time.Minute)},
		{ID: "book", Kind: "book", Title: "Book", Added: now.Add(-5 * time.Minute)},
		{ID: "photo", Kind: "photo", Title: "Photo", Added: now.Add(-6 * time.Minute)},
	}
	source := &testSource{
		result:   browseResult(t, nil, items),
		shell:    NewShell("Player", true, "Owner", "viewer", []navigation.Link{{Name: "Home"}}, []navigation.Link{{Name: "More"}}, true),
		cards:    CatalogProjection[string, string, string]{Shows: []string{"show"}, PlaylistCards: []string{"playlist"}, CustomCollectionCards: []string{"custom"}, DefaultCollectionCards: []string{"default"}},
		personal: PersonalProjection[string]{Continue: []string{"resume"}, List: []library.Item{items[0]}, Played: []library.Item{items[5]}, CollectionCount: 1, PlaylistCount: 1},
		artwork:  map[string]bool{"movie": true, "episode": true},
	}
	view := &testView{}
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "http://example.test/?sort=title", nil)
	response := httptest.NewRecorder()
	NewHandler[string, string, string, string](source, view, true, func(http.ResponseWriter, *http.Request, string, int) { t.Fatal("unexpected failure") })(response, request)

	page, ok := view.data.(Page[string, string, string, string])
	if !ok {
		t.Fatalf("page type = %T", view.data)
	}
	if response.Header().Get("Content-Type") != "text/html; charset=utf-8" || response.Header().Get("Vary") != InfiniteLibraryHeader || view.fullCalls != 1 || view.templateCalls != 0 {
		t.Fatalf("response=%v view=%+v", response.Header(), view)
	}
	if page.ServerName != "Player" || !page.Owner || page.ViewerName != "Owner" || !page.CanLogout || !page.UpdateAvailable || !page.CommandMenu || !page.TMDB {
		t.Fatalf("shell page = %+v", page)
	}
	if len(page.Items) != 1 || len(page.Music) != 1 || len(page.Albums) != 1 || len(page.Audiobooks) != 1 || len(page.Books) != 1 || len(page.Photos) != 1 || len(page.Shows) != 1 {
		t.Fatalf("media page = %+v", page)
	}
	if len(page.Continue) != 1 || len(page.List) != 1 || len(page.Recent) != len(items) || len(page.Played) != 1 || len(page.Destinations) != 3 || source.personalCalls != 1 {
		t.Fatalf("landing page = %+v", page)
	}
	if page.SearchClear != "/?sort=title" || len(page.NavigationPrimary) != 1 || len(page.NavigationMore) != 1 || page.HomeArtwork["episode"] != "episode" || page.HomeShowTitle["episode"] != "Series" {
		t.Fatalf("page links and artwork = %+v", page)
	}
}

func TestHandlerPreservesFragmentAndErrorBoundaries(t *testing.T) { //nolint:cyclop,gocognit // A table keeps every render and input failure boundary together.
	internal := errors.New("index failed")
	tests := []struct {
		name       string
		header     []string
		sourceErr  error
		viewErr    error
		wantStatus int
		fragment   bool
	}{
		{name: "bad header", header: []string{"bad"}, wantStatus: http.StatusBadRequest},
		{name: "repeated header", header: []string{"1", "1"}, wantStatus: http.StatusBadRequest},
		{name: "bad browse", sourceErr: catalog.ErrInvalidBrowse, wantStatus: http.StatusBadRequest},
		{name: "index failure", sourceErr: internal, wantStatus: http.StatusInternalServerError},
		{name: "full render failure", viewErr: internal},
		{name: "fragment render failure", header: []string{"1"}, viewErr: internal, fragment: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			source := &testSource{result: browseResult(t, url.Values{"view": {"movies"}}, []library.Item{{ID: "movie", Kind: "video", Title: "Movie"}}), err: test.sourceErr}
			view := &testView{err: test.viewErr}
			failure := failureRecord{}
			request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "http://example.test/?view=movies", nil)
			for _, value := range test.header {
				request.Header.Add(InfiniteLibraryHeader, value)
			}
			NewHandler[string, string, string, string](source, view, false, func(_ http.ResponseWriter, _ *http.Request, message string, status int) {
				failure = failureRecord{message, status}
			})(httptest.NewRecorder(), request)
			if failure.status != test.wantStatus {
				t.Fatalf("failure=%+v", failure)
			}
			if test.wantStatus != 0 && (view.fullCalls != 0 || view.templateCalls != 0 || failure.message == "") {
				t.Fatalf("failure boundary: failure=%+v view=%+v", failure, view)
			}
			if test.wantStatus == 0 && test.fragment != (view.templateCalls == 1) {
				t.Fatalf("fragment=%v view=%+v", test.fragment, view)
			}
			if source.personalCalls != 0 {
				t.Fatalf("non-landing page used personal state %d times", source.personalCalls)
			}
		})
	}
}
