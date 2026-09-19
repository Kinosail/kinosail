package catalog

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail/packages/library"
)

func TestPlaylistHandlersProjectBrowseThroughPublicPage(t *testing.T) {
	fixture := newPlaylistHTTPFixture()
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/playlist/Favorites?q=arrival", nil)
	request.SetPathValue("name", "Favorites")
	response := httptest.NewRecorder()

	fixture.handlers.Browse(response, request)

	if response.Code != http.StatusOK || response.Header().Get("Content-Type") != "text/html; charset=utf-8" {
		t.Fatalf("browse response = %d, %q", response.Code, response.Header().Get("Content-Type"))
	}
	if fixture.renderCalls != 1 || fixture.failureCalls != 0 || fixture.notFoundCalls != 0 {
		t.Fatalf("browse callbacks = render %d, failure %d, not found %d", fixture.renderCalls, fixture.failureCalls, fixture.notFoundCalls)
	}
	if fixture.page.Name != "Favorites" || fixture.page.Query != "arrival" || fixture.page.Mode != "Manual" || fixture.page.Smart || fixture.page.ItemCount != 2 {
		t.Fatalf("browse page = %#v", fixture.page)
	}
	if len(fixture.page.Items) != 2 || fixture.page.Items[0].CanEarlier || !fixture.page.Items[0].CanLater || !fixture.page.Items[1].CanEarlier || fixture.page.Items[1].CanLater {
		t.Fatalf("browse item navigation = %#v", fixture.page.Items)
	}
	if len(fixture.page.Candidates) != 1 || fixture.page.Candidates[0].ID != "0011223344556677" {
		t.Fatalf("browse candidates = %#v", fixture.page.Candidates)
	}
}

func TestPlaylistHandlersRejectBrowseAndMembershipInputBeforeSideEffects(t *testing.T) {
	fixture := newPlaylistHTTPFixture()
	fixture.handlers.Browse(httptest.NewRecorder(), playlistRequest(http.MethodGet, "/playlist/Missing", "", "Missing"))
	if fixture.notFoundCalls != 1 || fixture.renderCalls != 0 || fixture.playlistCalls != 0 {
		t.Fatalf("unknown playlist callbacks = not found %d, render %d, playlist %d", fixture.notFoundCalls, fixture.renderCalls, fixture.playlistCalls)
	}

	for _, test := range []struct {
		name, target, body string
	}{
		{"query key", "/playlist/Favorites?unexpected=x", "included=true"},
		{"invalid boolean", "/playlist/Favorites/item", "included=maybe"},
		{"duplicate boolean", "/playlist/Favorites/item", "included=true&included=false"},
		{"unknown form key", "/playlist/Favorites/item", "included=true&unexpected=x"},
		{"oversized query", "/playlist/Favorites/item", "included=true&q=" + strings.Repeat("x", 201)},
	} {
		request := playlistRequest(http.MethodPost, test.target, test.body, "Favorites")
		request.SetPathValue("id", "0011223344556677")
		fixture.handlers.ManageItem(httptest.NewRecorder(), request)
	}
	if fixture.visibleItemCalls != 0 || fixture.setCalls != 0 {
		t.Fatalf("rejected membership input crossed side-effect boundary: visible %d, set %d", fixture.visibleItemCalls, fixture.setCalls)
	}

	request := playlistRequest(http.MethodPost, "/playlist/Favorites/item", "included=true&q=arrival", "Favorites")
	request.SetPathValue("id", "0011223344556677")
	response := httptest.NewRecorder()
	fixture.handlers.ManageItem(response, request)
	if response.Code != http.StatusSeeOther || response.Header().Get("Location") != "/playlist/Favorites?q=arrival#playlist-search" {
		t.Fatalf("valid membership response = %d, %q", response.Code, response.Header().Get("Location"))
	}
	if fixture.setCalls != 1 || fixture.savedViewer != "viewer" || fixture.savedName != "Favorites" || fixture.savedID != "0011223344556677" || !fixture.savedIncluded {
		t.Fatalf("membership save = calls %d, viewer %q, name %q, id %q, included %t", fixture.setCalls, fixture.savedViewer, fixture.savedName, fixture.savedID, fixture.savedIncluded)
	}
}

func TestPlaylistHandlersValidateOrderBeforeMutation(t *testing.T) {
	fixture := newPlaylistHTTPFixture()
	for _, body := range []string{"id=fedcba9876543210&direction=sideways", "id=fedcba9876543210&direction=down&unexpected=x", "id=fedcba9876543210&id=8899aabbccddeeff&direction=down"} {
		request := playlistRequest(http.MethodPost, "/playlist/Favorites/order", body, "Favorites")
		request.SetPathValue("name", "Favorites")
		fixture.handlers.Order(httptest.NewRecorder(), request)
	}
	if fixture.playlistCalls != 0 || fixture.orderCalls != 0 {
		t.Fatalf("rejected order crossed mutation boundary: playlist %d, order %d", fixture.playlistCalls, fixture.orderCalls)
	}

	request := playlistRequest(http.MethodPost, "/playlist/Favorites/order", "id=fedcba9876543210&direction=down", "Favorites")
	request.SetPathValue("name", "Favorites")
	response := httptest.NewRecorder()
	fixture.handlers.Order(response, request)
	if response.Code != http.StatusSeeOther || response.Header().Get("Location") != "/playlist/Favorites" {
		t.Fatalf("valid order response = %d, %q", response.Code, response.Header().Get("Location"))
	}
	want := []string{"8899aabbccddeeff", "fedcba9876543210"}
	if fixture.orderCalls != 1 || fixture.savedViewer != "viewer" || fixture.savedName != "Favorites" || !reflect.DeepEqual(fixture.orderedIDs, want) {
		t.Fatalf("ordered playlist = calls %d, viewer %q, name %q, ids %#v", fixture.orderCalls, fixture.savedViewer, fixture.savedName, fixture.orderedIDs)
	}
}

func TestPlaylistHandlersReportRenderAndMutationFailures(t *testing.T) {
	fixture := newPlaylistHTTPFixture()
	fixture.renderErr = errors.New("render playlist")
	request := playlistRequest(http.MethodGet, "/playlist/Favorites", "", "Favorites")
	fixture.handlers.Browse(httptest.NewRecorder(), request)
	if fixture.failureStatus != http.StatusInternalServerError || fixture.failureMessage != "render playlist" {
		t.Fatalf("render failure = %q, %d", fixture.failureMessage, fixture.failureStatus)
	}

	fixture = newPlaylistHTTPFixture()
	fixture.orderErr = errors.New("save order")
	request = playlistRequest(http.MethodPost, "/playlist/Favorites/order", "id=fedcba9876543210&direction=down", "Favorites")
	fixture.handlers.Order(httptest.NewRecorder(), request)
	if fixture.failureStatus != http.StatusBadRequest || fixture.failureMessage != "save order" || fixture.orderCalls != 1 {
		t.Fatalf("order failure = %q, %d, calls %d", fixture.failureMessage, fixture.failureStatus, fixture.orderCalls)
	}
}

type playlistHTTPFixture struct {
	handlers PlaylistHandlers
	items    []library.Item
	members  []library.Item
	page     PlaylistPage

	renderErr, orderErr                                   error
	renderCalls, failureCalls, notFoundCalls              int
	playlistCalls, visibleItemCalls, setCalls, orderCalls int
	failureMessage                                        string
	failureStatus                                         int
	savedViewer, savedName, savedID                       string
	savedIncluded                                         bool
	orderedIDs                                            []string
}

func newPlaylistHTTPFixture() *playlistHTTPFixture {
	fixture := &playlistHTTPFixture{
		items: []library.Item{
			{ID: "fedcba9876543210", Title: "Favorites"},
			{ID: "0011223344556677", Title: "Arrival"},
			{ID: "8899aabbccddeeff", Title: "Other"},
		},
		members: []library.Item{
			{ID: "fedcba9876543210", Title: "Favorites"},
			{ID: "8899aabbccddeeff", Title: "Other"},
		},
	}
	fixture.handlers = NewPlaylistHandlers(PlaylistHandlersConfig{
		VisibleLibrary: func(*http.Request) ([]library.Item, error) { return fixture.items, nil },
		PlaylistNames:  func(*http.Request) []string { return []string{"Favorites"} },
		Playlist: func(*http.Request, string, []library.Item) []library.Item {
			fixture.playlistCalls++
			return append([]library.Item(nil), fixture.members...)
		},
		PlaylistRule: func(*http.Request, string) (PlaylistRule, bool) { return PlaylistRule{}, false },
		VisibleItem: func(_ *http.Request, id string) (library.Item, bool) {
			fixture.visibleItemCalls++
			for _, item := range fixture.items {
				if item.ID == id {
					return item, true
				}
			}
			return library.Item{}, false
		},
		Viewer: func(*http.Request) string { return "viewer" },
		SetPlaylist: func(_ context.Context, viewer, name, id string, included bool) error {
			fixture.setCalls++
			fixture.savedViewer, fixture.savedName, fixture.savedID, fixture.savedIncluded = viewer, name, id, included
			return nil
		},
		Order: func(_ context.Context, viewer, name string, ids []string) error {
			fixture.orderCalls++
			fixture.savedViewer, fixture.savedName = viewer, name
			fixture.orderedIDs = append([]string(nil), ids...)
			return fixture.orderErr
		},
		Render: func(_ http.ResponseWriter, _ *http.Request, page PlaylistPage) error {
			fixture.renderCalls++
			fixture.page = page
			return fixture.renderErr
		},
		Failure: func(writer http.ResponseWriter, _ *http.Request, message string, status int) {
			fixture.failureCalls++
			fixture.failureMessage, fixture.failureStatus = message, status
			writer.WriteHeader(status)
		},
		NotFound: func(writer http.ResponseWriter, _ *http.Request) {
			fixture.notFoundCalls++
			writer.WriteHeader(http.StatusNotFound)
		},
	})
	return fixture
}

func playlistRequest(method, target, body string, paths ...string) *http.Request {
	request := httptest.NewRequestWithContext(context.Background(), method, target, strings.NewReader(body))
	if method == http.MethodPost {
		request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	if len(paths) > 0 {
		request.SetPathValue("name", paths[0])
	}
	return request
}
