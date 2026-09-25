package catalog

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail/packages/library"
)

func TestDetailPresentationValidation(t *testing.T) {
	t.Parallel()
	invalidUTF8 := string([]byte{0xff})
	tests := []DetailPresentation{
		{},
		{Product: " Kinosail Player", ThemeVersion: "6", StyleVersion: "75"},
		{Product: strings.Repeat("p", 65), ThemeVersion: "6", StyleVersion: "75"},
		{Product: invalidUTF8, ThemeVersion: "6", StyleVersion: "75"},
		{Product: "Kinosail\nPlayer", ThemeVersion: "6", StyleVersion: "75"},
		{Product: "Kinosail Player", StyleVersion: "75"},
		{Product: "Kinosail Player", ThemeVersion: strings.Repeat("v", 33), StyleVersion: "75"},
		{Product: "Kinosail Player", ThemeVersion: "../6", StyleVersion: "75"},
		{Product: "Kinosail Player", ThemeVersion: "6", StyleVersion: "75?"},
	}
	for _, presentation := range tests {
		if sources, err := NewDetailSources(presentation); !errors.Is(err, ErrInvalidDetailPresentation) || sources != (DetailSources{}) {
			t.Fatalf("NewDetailSources(%q) = %#v, %v", presentation.Product, sources, err)
		}
	}
	sources, err := NewDetailSources(DetailPresentation{"Kinosail Édition", "v1.2-a_b", "75"})
	if err != nil || !strings.Contains(sources.Album, "Kinosail Édition") {
		t.Fatalf("valid presentation = %#v, %v", sources, err)
	}
}

func TestMustDetailSourcesRejectsInvalidStaticConfiguration(t *testing.T) {
	t.Parallel()
	defer func() {
		if recovered := recover(); !errors.Is(recovered.(error), ErrInvalidDetailPresentation) {
			t.Fatalf("panic = %v", recovered)
		}
	}()
	MustDetailSources(DetailPresentation{})
}

func TestDetailHandlersRejectIDsBeforeDataCallbacks(t *testing.T) {
	t.Parallel()
	index := &detailIndexFixture{}
	lists := &detailListsFixture{}
	album, book := &detailViewFixture{}, &detailViewFixture{}
	viewerCalls, notFound := 0, 0
	details := NewDetailHTTP(album, book, func(*http.Request) DetailViewer {
		viewerCalls++
		return DetailViewer{}
	}, func(http.ResponseWriter, *http.Request, string, int) {
		t.Fatal("unexpected failure callback")
	}, func(http.ResponseWriter, *http.Request) {
		notFound++
	})
	for _, test := range []struct {
		name, id string
		handler  http.HandlerFunc
	}{
		{"empty album", "", details.Album(index)},
		{"oversized book", strings.Repeat("a", maximumDetailIDBytes+1), details.Book(index, lists)},
		{"invalid UTF-8 album", string([]byte{0xff}), details.Album(index)},
	} {
		request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
		request.SetPathValue("id", test.id)
		test.handler(httptest.NewRecorder(), request)
	}
	if index.calls != 0 || lists.calls != 0 || album.calls != 0 || book.calls != 0 || viewerCalls != 0 || notFound != 3 {
		t.Fatalf("callbacks = index %d, lists %d, views %d/%d, viewer %d, not found %d", index.calls, lists.calls, album.calls, book.calls, viewerCalls, notFound)
	}
}

func TestAlbumHandlerProjectsAndReportsResults(t *testing.T) {
	t.Parallel()
	items := []library.Item{{ID: "second", Kind: "audio", Title: "Second", Artist: "Artist", Album: "Record", Track: 2}, {ID: "first", Kind: "audio", Title: "First", Artist: "Artist", Album: "Record", Track: 1}}
	_, albums := library.OrganizeMusic(items)
	tests := []struct {
		name, id                            string
		renderErr                           error
		wantView, wantNotFound, wantFailure int
	}{
		{"missing", "missing", nil, 0, 1, 0},
		{"found", albums[0].ID, nil, 1, 0, 0},
		{"render failure", albums[0].ID, errors.New("render album"), 1, 0, 1},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			index := &detailIndexFixture{items: items}
			view := &detailViewFixture{err: test.renderErr}
			failure, notFound := 0, 0
			details := NewDetailHTTP(view, &detailViewFixture{}, func(*http.Request) DetailViewer { return DetailViewer{} }, func(_ http.ResponseWriter, _ *http.Request, message string, status int) {
				failure++
				if message != "render album" || status != http.StatusInternalServerError {
					t.Fatalf("failure = %q, %d", message, status)
				}
			}, func(http.ResponseWriter, *http.Request) { notFound++ })
			request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/album/"+test.id, nil)
			request.SetPathValue("id", test.id)
			response := httptest.NewRecorder()
			details.Album(index).ServeHTTP(response, request)
			if view.calls != test.wantView || notFound != test.wantNotFound || failure != test.wantFailure || index.calls != 1 {
				t.Fatalf("calls = view %d, not found %d, failure %d, index %d", view.calls, notFound, failure, index.calls)
			}
			if test.wantView == 1 {
				checkAlbumProjection(t, view.data.(AlbumPage), response.Header())
			}
		})
	}
}

func checkAlbumProjection(t *testing.T, data AlbumPage, header http.Header) {
	t.Helper()
	if data.First == nil || data.First.ID != "first" || header.Get("Content-Type") != "text/html; charset=utf-8" {
		t.Fatalf("album projection = %#v, header %q", data, header)
	}
}

func TestBookHandlerProjectsCurationAndAccess(t *testing.T) {
	t.Parallel()
	book := library.Item{ID: "book", Kind: "book", Title: "Title", Plot: "Plot", Container: "CBZ", Year: "2026", Artwork: "cover.jpg"}
	index := &detailIndexFixture{items: []library.Item{book}, item: book, found: true}
	lists := &detailListsFixture{playlistNames: []string{"Included", "Open"}, collectionNames: []string{"Shelf"}, listed: true}
	view := &detailViewFixture{}
	details := NewDetailHTTP(&detailViewFixture{}, view, func(*http.Request) DetailViewer {
		return DetailViewer{Downloads: true}
	}, func(http.ResponseWriter, *http.Request, string, int) {
		t.Fatal("unexpected failure callback")
	}, func(http.ResponseWriter, *http.Request) {
		t.Fatal("unexpected not-found callback")
	})
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/book/book", nil)
	request.SetPathValue("id", "book")
	response := httptest.NewRecorder()
	details.Book(index, lists).ServeHTTP(response, request)
	want := BookPage{
		ID: "book", Title: "Title", Plot: "Plot", Container: "CBZ", Year: "2026", Artwork: true, CanDownload: true, Listed: true,
		Playlists: []DetailOption{{"Included", true}, {"Open", false}}, Collections: []DetailOption{{"Shelf", true}},
	}
	if got := view.data.(BookPage); !reflect.DeepEqual(got, want) || view.calls != 1 || index.calls != 2 || lists.calls != 6 {
		t.Fatalf("book projection = %#v; calls view=%d index=%d lists=%d", got, view.calls, index.calls, lists.calls)
	}
}

func TestBookHandlerStopsBeforeCurationForMissingOrWrongKind(t *testing.T) {
	t.Parallel()
	for _, index := range []*detailIndexFixture{{}, {item: library.Item{ID: "video", Kind: "video"}, found: true}} {
		lists, view, notFound := &detailListsFixture{}, &detailViewFixture{}, 0
		details := NewDetailHTTP(&detailViewFixture{}, view, func(*http.Request) DetailViewer { t.Fatal("viewer called"); return DetailViewer{} }, func(http.ResponseWriter, *http.Request, string, int) { t.Fatal("failure called") }, func(http.ResponseWriter, *http.Request) { notFound++ })
		request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/book/value", nil)
		request.SetPathValue("id", "value")
		details.Book(index, lists).ServeHTTP(httptest.NewRecorder(), request)
		if notFound != 1 || index.calls != 1 || lists.calls != 0 || view.calls != 0 {
			t.Fatalf("callbacks = not found %d, index %d, lists %d, view %d", notFound, index.calls, lists.calls, view.calls)
		}
	}
}

type detailViewFixture struct {
	data  any
	err   error
	calls int
}

func (view *detailViewFixture) Execute(_ http.ResponseWriter, _ *http.Request, anyData any) error {
	view.calls++
	view.data = anyData
	return view.err
}

type detailIndexFixture struct {
	items []library.Item
	item  library.Item
	found bool
	calls int
}

func (index *detailIndexFixture) VisibleLibrary(*http.Request) []library.Item {
	index.calls++
	return index.items
}

func (index *detailIndexFixture) VisibleItem(*http.Request, string) (library.Item, bool) {
	index.calls++
	return index.item, index.found
}

type detailListsFixture struct {
	playlistNames, collectionNames []string
	listed                         bool
	calls                          int
}

func (lists *detailListsFixture) EditablePlaylistNames(*http.Request) []string {
	lists.calls++
	return lists.playlistNames
}

func (lists *detailListsFixture) Playlist(_ *http.Request, name string, _ []library.Item) []library.Item {
	lists.calls++
	if name == "Included" {
		return []library.Item{{ID: "book"}}
	}
	return nil
}

func (lists *detailListsFixture) CollectionNames([]library.Item) []string {
	lists.calls++
	return lists.collectionNames
}

func (lists *detailListsFixture) Collection(string, []library.Item) []library.Item {
	lists.calls++
	return []library.Item{{ID: "book"}}
}

func (lists *detailListsFixture) Has(*http.Request, string) bool {
	lists.calls++
	return lists.listed
}
