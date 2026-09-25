package catalogapi

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/MikeO7/kinosail/packages/catalog"
	"github.com/MikeO7/kinosail/packages/library"
)

func TestProjectItemUsesPlayerCanonicalPublicPaths(t *testing.T) { //nolint:cyclop // One projection contract remains below the repository complexity ceiling.
	added := time.Date(2026, time.September, 4, 1, 2, 3, 0, time.UTC)
	item := library.Item{ID: "episode", Kind: "video", Title: "Pilot", SortTitle: "Pilot, The", Year: "2026", Plot: "Plot", Rating: "PG", Tagline: "Tagline", Genres: "Drama", Director: "Director", Studio: "Studio", Artist: "Artist", Album: "Album", Track: 2, Show: "Series", Season: 1, Episode: 2, Artwork: "episode.jpg", ShowBackdrop: "show.jpg", Subtitles: []string{"one", "two"}, Container: "MKV", Size: 42, Added: added, Cast: []library.Person{{Name: "Actor", Role: "Lead", Image: "actor.jpg"}, {Name: "Voice"}}}
	progress := catalog.PlaybackState{Seconds: 12}
	result := ProjectItem(item, progress, ItemAccess{Stream: true, Download: true})
	if result.Stream != "/media/episode" || result.Download != "/download/episode" || result.Artwork != "/art/episode?variant=episode" || result.Backdrop != "/backdrop/episode" || result.Progress != progress || result.Subtitles != 2 || result.Added != added {
		t.Fatalf("projection = %#v", result)
	}
	if !reflect.DeepEqual(result.Cast, []ClientPerson{{Name: "Actor", Role: "Lead", Image: "/person/episode/0"}, {Name: "Voice"}}) {
		t.Fatalf("cast = %#v", result.Cast)
	}
	plain := ProjectItem(library.Item{ID: "movie", Title: "Movie"}, catalog.PlaybackState{}, ItemAccess{})
	if plain.Stream != "" || plain.Download != "" || plain.Artwork != "" || plain.Backdrop != "" || ArtworkURLForItem(library.Item{ID: "movie"}) != "/art/movie" {
		t.Fatalf("plain projection = %#v", plain)
	}
	posterOnly := ProjectItem(library.Item{ID: "poster", Kind: "video", Artwork: "poster.jpg"}, catalog.PlaybackState{}, ItemAccess{})
	if posterOnly.Artwork != "/art/poster" || posterOnly.Backdrop != "" {
		t.Fatalf("portrait poster advertised as landscape: %#v", posterOnly)
	}
}

func TestBrowseProjectionHelpersCoverAvailableAndMissingMedia(t *testing.T) { //nolint:cyclop // One projection matrix remains below the repository complexity ceiling.
	if ArtworkURL("") != "" || ArtworkURL("item") != "/art/item" {
		t.Fatal("artwork route selection changed")
	}
	if BackdropURL("", true) != "" || BackdropURL("item", false) != "" || BackdropURL("item", true) != "/backdrop/item" {
		t.Fatal("backdrop route selection changed")
	}
	if id := ShowBackdropID(library.Show{ArtworkID: "art"}); id != "art" {
		t.Fatalf("artwork backdrop ID = %q", id)
	}
	if id := ShowBackdropID(library.Show{ArtworkID: "art", Backdrop: "backdrop", Episodes: []library.Item{{ID: "episode"}}}); id != "episode" {
		t.Fatalf("landscape backdrop priority = %q", id)
	}
	if id := ShowBackdropID(library.Show{Backdrop: "backdrop", Episodes: []library.Item{{ID: "episode"}}}); id != "episode" {
		t.Fatalf("episode backdrop ID = %q", id)
	}
	if id := ShowBackdropID(library.Show{}); id != "" {
		t.Fatalf("empty backdrop ID = %q", id)
	}
	if queue, found := AudioQueue(mediaFixture(), "track-two"); !found || len(queue) != 1 || queue[0].ID != "track-two" {
		t.Fatalf("audio queue = %#v, %t", queue, found)
	}
	if _, found := AudioQueue(mediaFixture(), "missing"); found {
		t.Fatal("missing track produced an audio queue")
	}
}

func TestPlaylistDocumentValidationBoundaries(t *testing.T) { //nolint:cyclop // One bounded document table covers every independent validation rule.
	valid := `{"format":"kinosail.playlist","version":1,"name":" Queue ","ids":["one"]}`
	document, err := DecodePlaylistDocument(valid)
	if err != nil || document.Name != " Queue " {
		t.Fatalf("decode = %#v, %v", document, err)
	}
	for _, raw := range []string{"", strings.Repeat("x", (1<<20)+1), "{", `{"name":"Queue","unknown":true}`, `{"name":"Queue"}{}`} {
		if _, err := DecodePlaylistDocument(raw); err == nil {
			t.Fatalf("DecodePlaylistDocument(%q) succeeded", raw[:min(len(raw), 40)])
		}
	}

	if name, err := ValidatePlaylist(" Queue ", []string{"one", "two"}); err != nil || name != "Queue" {
		t.Fatalf("valid playlist = %q, %v", name, err)
	}
	invalid := []struct {
		name string
		ids  []string
	}{
		{"", nil},
		{"Queue", make([]string, 201)},
		{"Queue", []string{""}},
		{"Queue", []string{strings.Repeat("x", 257)}},
		{"Queue", []string{"same", "same"}},
	}
	for _, test := range invalid {
		if _, err := ValidatePlaylist(test.name, test.ids); err == nil {
			t.Fatalf("ValidatePlaylist(%q, %d IDs) succeeded", test.name, len(test.ids))
		}
	}
	if name, err := ValidatePlaylistDocument(PlaylistDocument{Name: "Legacy"}); err != nil || name != "Legacy" {
		t.Fatalf("legacy document = %q, %v", name, err)
	}
	if _, err := ValidatePlaylistDocument(PlaylistDocument{Format: PlaylistDocumentFormat, Version: 2, Name: "Queue"}); err == nil {
		t.Fatal("unsupported document version succeeded")
	}
	if name, err := ValidatePlaylistDocument(document); err != nil || name != "Queue" {
		t.Fatalf("versioned document = %q, %v", name, err)
	}
}

func TestCreatePlaylistDocumentValidatesBeforePersistence(t *testing.T) { //nolint:cyclop // One adapter test proves validation, persistence, and error ordering.
	document := PlaylistDocument{Format: PlaylistDocumentFormat, Version: 1, Name: " Queue ", IDs: []string{"one"}}
	created := 0
	visible := func(id string) (library.Item, bool) { return library.Item{ID: id}, id == "one" }
	create := func(_ context.Context, viewer, name string, ids ...string) error {
		created++
		if viewer != "viewer" || name != "Queue" || !reflect.DeepEqual(ids, []string{"one"}) {
			t.Fatalf("create input = %q %q %#v", viewer, name, ids)
		}
		return nil
	}
	if name, err := CreatePlaylistDocument(t.Context(), "viewer", document, visible, create); err != nil || name != "Queue" || created != 1 {
		t.Fatalf("create = %q, %v, calls=%d", name, err, created)
	}
	if _, err := CreatePlaylistDocument(t.Context(), "viewer", PlaylistDocument{}, visible, create); err == nil || created != 1 {
		t.Fatalf("invalid create = %v, calls=%d", err, created)
	}
	unavailable := document
	unavailable.IDs = []string{"missing"}
	if _, err := CreatePlaylistDocument(t.Context(), "viewer", unavailable, visible, create); err == nil || created != 1 {
		t.Fatalf("unavailable create = %v, calls=%d", err, created)
	}
	createErr := errors.New("disk")
	if _, err := CreatePlaylistDocument(t.Context(), "viewer", document, visible, func(context.Context, string, string, ...string) error { return createErr }); !errors.Is(err, createErr) {
		t.Fatalf("create error = %v", err)
	}
}

func TestNewListHandlersImportsPortableDocuments(t *testing.T) {
	items := mediaFixture()
	index := &mediaIndexStub{items: items, visible: items[0], found: true}
	lists := newMediaListsStub(items)
	handlers := NewListHandlers(index, lists, func(*http.Request) string { return "viewer" }, func(writer http.ResponseWriter, _ *http.Request, message string, status int) {
		http.Error(writer, message, status)
	}, func(writer http.ResponseWriter, _ *http.Request) { writer.WriteHeader(http.StatusNotFound) })
	call := func(document string) *httptest.ResponseRecorder {
		request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/playlists", strings.NewReader(url.Values{"document": {document}}.Encode()))
		request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		response := httptest.NewRecorder()
		handlers.CreatePlaylist(response, request)
		return response
	}
	if response := call(`{"format":"kinosail.playlist","version":1,"name":"Queue","ids":["movie"]}`); response.Code != http.StatusSeeOther || lists.creates != 1 {
		t.Fatalf("portable import = %d calls=%d body=%q", response.Code, lists.creates, response.Body.String())
	}
	if response := call(`{"unknown":true}`); response.Code != http.StatusBadRequest || lists.creates != 1 {
		t.Fatalf("invalid portable import = %d calls=%d body=%q", response.Code, lists.creates, response.Body.String())
	}
}

func TestExportPlaylistDocumentPreservesManualOrder(t *testing.T) {
	if _, err := ExportPlaylistDocument("Missing", nil, nil, false); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("missing export = %v", err)
	}
	if _, err := ExportPlaylistDocument("Smart", map[string]bool{"one": true}, nil, true); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("smart export = %v", err)
	}
	document, err := ExportPlaylistDocument("Queue", map[string]bool{"one": true, "two": true, "three": false}, []string{"two", "two"}, false)
	if err != nil || !reflect.DeepEqual(document, PlaylistDocument{Format: PlaylistDocumentFormat, Version: 1, Name: "Queue", IDs: []string{"two", "one"}}) {
		t.Fatalf("export = %#v, %v", document, err)
	}
}
