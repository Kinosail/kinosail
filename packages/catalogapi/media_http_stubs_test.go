package catalogapi

import (
	"context"
	"errors"
	"net/http"

	"github.com/MikeO7/kinosail/packages/catalog"
	"github.com/MikeO7/kinosail/packages/library"
	"github.com/MikeO7/kinosail/packages/playback"
)

type mediaProgressStub struct {
	state                  catalog.PlaybackState
	accepted               bool
	err, dismissErr        error
	itemsCalls, itemCalls  int
	dismisses, progressSet int
	lastSeconds            float64
}

func (stub *mediaProgressStub) ClientItem(_ *http.Request, item library.Item) any {
	stub.itemCalls++
	return map[string]string{"id": item.ID}
}

func (stub *mediaProgressStub) ClientItems(_ *http.Request, items []library.Item) any {
	stub.itemsCalls++
	result := make([]ClientItem, 0, len(items))
	for _, item := range items {
		result = append(result, ProjectItem(item, stub.state, ItemAccess{}))
	}
	return result
}

func (stub *mediaProgressStub) Dismiss(*http.Request, string) error {
	stub.dismisses++
	return stub.dismissErr
}

func (stub *mediaProgressStub) Get(*http.Request, string) catalog.PlaybackState { return stub.state }

func (stub *mediaProgressStub) History(_ *http.Request, items []library.Item) []library.Item {
	return items
}

func (stub *mediaProgressStub) SetRevision(_ *http.Request, _ string, seconds float64, _ *bool, _ string, _ uint64) (bool, error) {
	stub.progressSet++
	stub.lastSeconds = seconds
	stub.state.Seconds = seconds
	return stub.accepted, stub.err
}

type mediaListsStub struct {
	names                       []string
	selected                    []library.Item
	document                    PlaylistDocument
	err, exportErr              error
	creates, smart, orders      int
	deletes, listed, membership int
}

func newMediaListsStub(items []library.Item) *mediaListsStub {
	return &mediaListsStub{
		names:    []string{"Queue"},
		selected: items,
		document: PlaylistDocument{Format: PlaylistDocumentFormat, Version: 1, Name: "Queue", IDs: []string{"movie"}},
	}
}

func (stub *mediaListsStub) Create(context.Context, string, string, ...string) error {
	stub.creates++
	return stub.err
}

func (stub *mediaListsStub) CreateSmart(context.Context, string, string, catalog.PlaylistRule) error {
	stub.smart++
	return stub.err
}

func (stub *mediaListsStub) DeletePlaylist(context.Context, string, string) error {
	stub.deletes++
	return stub.err
}

func (stub *mediaListsStub) ExportPlaylistDocument(string, string) (PlaylistDocument, error) {
	return stub.document, stub.exportErr
}

func (stub *mediaListsStub) Has(*http.Request, string) bool { return true }

func (stub *mediaListsStub) Order(context.Context, string, string, []string) error {
	stub.orders++
	return stub.err
}

func (stub *mediaListsStub) Playlist(*http.Request, string, []library.Item) []library.Item {
	return stub.selected
}

func (stub *mediaListsStub) PlaylistNames(*http.Request) []string { return stub.names }

func (stub *mediaListsStub) PlaylistSummariesJSON(*http.Request, []library.Item, string) any {
	return []string{"summary"}
}

func (stub *mediaListsStub) SetListed(context.Context, string, string, bool) error {
	stub.listed++
	return stub.err
}

func (stub *mediaListsStub) SetPlaylist(context.Context, string, string, string, bool) error {
	stub.membership++
	return stub.err
}

func (stub *mediaListsStub) CollectionNames([]library.Item) []string { return []string{"Queue"} }

func (stub *mediaListsStub) CollectionSummaries([]library.Item) any { return []string{"summary"} }

func (stub *mediaListsStub) Collection(_ string, items []library.Item) []library.Item { return items }

func (stub *mediaListsStub) CreateCollection(context.Context, string) error { return stub.err }

func (stub *mediaListsStub) SetCollection(context.Context, string, library.Item, bool) error {
	return stub.err
}

func (stub *mediaListsStub) DeleteCollection(context.Context, string, []library.Item) error {
	return stub.err
}

func mediaFixture() []library.Item {
	return []library.Item{
		{ID: "movie", Kind: "video", Title: "Movie", Artwork: "movie.jpg"},
		{ID: "episode", Kind: "video", Title: "S01E01 · Pilot", Show: "show", ShowTitle: "Series", ShowArtwork: "show.jpg", ShowBackdrop: "backdrop.jpg", Season: 1, Episode: 1},
		{ID: "track-one", Kind: "audio", Title: "One", Album: "Album", Artist: "Artist", Track: 1, Artwork: "cover.jpg"},
		{ID: "track-two", Kind: "audio", Title: "Two", Album: "Album", Artist: "Artist", Track: 2, Artwork: "cover.jpg"},
	}
}

type mediaIndexStub struct {
	items   []library.Item
	visible library.Item
	found   bool
}

func (stub *mediaIndexStub) VisibleLibrary(*http.Request) []library.Item { return stub.items }

func (stub *mediaIndexStub) VisibleItem(*http.Request, string) (library.Item, bool) {
	return stub.visible, stub.found
}

func mediaHandlersForTest(items []library.Item) (MediaHandlers, *mediaIndexStub, *mediaProgressStub, *mediaListsStub) {
	index := &mediaIndexStub{items: items, visible: items[0], found: true}
	progress := &mediaProgressStub{accepted: true}
	lists := newMediaListsStub(items[:1])
	handlers := NewMediaHandlers(index, progress, lists, func(*http.Request) string { return "viewer" }, func(token string) (timeline playback.Timeline, err error) {
		if token == "bad" {
			return timeline, errors.New("bad token")
		}
		return playback.Timeline{SourceDuration: 100, Duration: 80, Omitted: []playback.Range{{Start: 0, End: 20}}}, nil
	}, func(error) int { return http.StatusTeapot })
	return handlers, index, progress, lists
}
