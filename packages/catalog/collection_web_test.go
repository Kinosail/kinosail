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

func TestCollectionWebHandlersProjectBrowseThroughPublicPage(t *testing.T) {
	fixture := newCollectionWebFixture()
	request := collectionWebRequest(http.MethodGet, "/collection/Shelf?q=arrival", "", "Shelf", "")
	response := httptest.NewRecorder()

	fixture.handlers.Browse(response, request)

	if response.Code != http.StatusOK || response.Header().Get("Content-Type") != "text/html; charset=utf-8" {
		t.Fatalf("browse response = %d, %q", response.Code, response.Header().Get("Content-Type"))
	}
	want := CollectionWebPage{
		Name: "Shelf", Query: "arrival", Items: fixture.members,
		Candidates: []library.Item{{ID: "0011223344556677", Title: "Arrival"}}, ItemCount: 2, Owner: true,
	}
	if fixture.renderCalls != 1 || !reflect.DeepEqual(fixture.page, want) || fixture.failureCalls != 0 || fixture.notFoundCalls != 0 {
		t.Fatalf("browse callbacks = render %d, page %#v, failure %d, not found %d", fixture.renderCalls, fixture.page, fixture.failureCalls, fixture.notFoundCalls)
	}

	fixture.handlers.Browse(httptest.NewRecorder(), collectionWebRequest(http.MethodGet, "/collection/Shelf?unexpected=x", "", "Shelf", ""))
	fixture.handlers.Browse(httptest.NewRecorder(), collectionWebRequest(http.MethodGet, "/collection/Missing", "", "Missing", ""))
	if fixture.renderCalls != 1 || fixture.failureStatus != http.StatusBadRequest || fixture.notFoundCalls != 1 {
		t.Fatalf("invalid browse callbacks = render %d, failure %d, not found %d", fixture.renderCalls, fixture.failureStatus, fixture.notFoundCalls)
	}
}

func TestCollectionWebHandlersRejectInvalidMutationsBeforeSideEffects(t *testing.T) {
	fixture := newCollectionWebFixture()
	for _, test := range []struct {
		target, body, name, id string
	}{
		{"/collection/Shelf/items/0011223344556677?unexpected=x", "included=true", "Shelf", "0011223344556677"},
		{"/collection/Shelf/items/0011223344556677", "included=maybe", "Shelf", "0011223344556677"},
		{"/collection/Shelf/items/0011223344556677", "included=true&included=false", "Shelf", "0011223344556677"},
		{"/collection/Shelf/items/0011223344556677", "included=true&unexpected=x", "Shelf", "0011223344556677"},
		{"/collection/bad/name/items/0011223344556677", "included=true", "bad/name", "0011223344556677"},
	} {
		fixture.handlers.ManageItem(httptest.NewRecorder(), collectionWebRequest(http.MethodPost, test.target, test.body, test.name, test.id))
	}
	if fixture.visibleItemCalls != 0 || fixture.setCalls != 0 {
		t.Fatalf("rejected membership input crossed side-effect boundary: visible %d, set %d", fixture.visibleItemCalls, fixture.setCalls)
	}

	request := collectionWebRequest(http.MethodPost, "/collection/Shelf/items/0011223344556677", "included=true&q=arrival", "Shelf", "0011223344556677")
	response := httptest.NewRecorder()
	fixture.handlers.ManageItem(response, request)
	if response.Code != http.StatusSeeOther || response.Header().Get("Location") != "/collection/Shelf?q=arrival#collection-search" || fixture.setCalls != 1 {
		t.Fatalf("valid membership = %d, %q, set calls %d", response.Code, response.Header().Get("Location"), fixture.setCalls)
	}

	for _, test := range []struct{ target, body, name, id string }{
		{"/collections", "name=", "", ""},
		{"/collections", "name=bad%2Fname", "", ""},
		{"/collections", "name=" + strings.Repeat("x", 65), "", ""},
		{"/collection/bad/name/0011223344556677", "included=true", "bad/name", "0011223344556677"},
		{"/collection/Shelf/not-an-id", "included=true", "Shelf", "not-an-id"},
	} {
		fixture.handlers.Create(httptest.NewRecorder(), collectionWebRequest(http.MethodPost, test.target, test.body, test.name, test.id))
		fixture.handlers.Save(httptest.NewRecorder(), collectionWebRequest(http.MethodPost, test.target, test.body, test.name, test.id))
	}
	if fixture.createCalls != 0 || fixture.visibleItemCalls != 1 || fixture.setCalls != 1 {
		t.Fatalf("rejected create/save crossed side-effect boundary: create %d, visible %d, set %d", fixture.createCalls, fixture.visibleItemCalls, fixture.setCalls)
	}

	fixture.handlers.Delete(httptest.NewRecorder(), collectionWebRequest(http.MethodPost, "/collection/bad/name/delete", "", "bad/name", ""))
	if fixture.snapshotCalls != 0 || fixture.deleteCalls != 0 {
		t.Fatalf("rejected delete crossed side-effect boundary: snapshot %d, delete %d", fixture.snapshotCalls, fixture.deleteCalls)
	}
}

func TestCollectionWebHandlersCompleteMutationsAndFailures(t *testing.T) {
	fixture := newCollectionWebFixture()
	request := collectionWebRequest(http.MethodPost, "/collections", "name=New+Shelf", "", "")
	response := httptest.NewRecorder()
	fixture.handlers.Create(response, request)
	if response.Code != http.StatusSeeOther || response.Header().Get("Location") != "/collection/New%20Shelf" || fixture.createCalls != 1 {
		t.Fatalf("create = %d, %q, calls %d", response.Code, response.Header().Get("Location"), fixture.createCalls)
	}

	request = collectionWebRequest(http.MethodPost, "/collection/Shelf/0011223344556677", "included=true", "Shelf", "0011223344556677")
	response = httptest.NewRecorder()
	fixture.handlers.Save(response, request)
	if response.Code != http.StatusSeeOther || response.Header().Get("Location") != "/watch/0011223344556677" {
		t.Fatalf("save = %d, %q", response.Code, response.Header().Get("Location"))
	}

	request = collectionWebRequest(http.MethodPost, "/collection/Shelf/delete", "", "Shelf", "")
	response = httptest.NewRecorder()
	fixture.handlers.Delete(response, request)
	if response.Code != http.StatusSeeOther || response.Header().Get("Location") != "/" || fixture.snapshotCalls != 1 || fixture.deleteCalls != 1 {
		t.Fatalf("delete = %d, %q, snapshot %d, calls %d", response.Code, response.Header().Get("Location"), fixture.snapshotCalls, fixture.deleteCalls)
	}

	fixture = newCollectionWebFixture()
	fixture.renderErr = errors.New("render Collection")
	fixture.handlers.Browse(httptest.NewRecorder(), collectionWebRequest(http.MethodGet, "/collection/Shelf", "", "Shelf", ""))
	if fixture.failureStatus != http.StatusInternalServerError || fixture.failureMessage != "render Collection" {
		t.Fatalf("render failure = %q, %d", fixture.failureMessage, fixture.failureStatus)
	}

	fixture = newCollectionWebFixture()
	fixture.setErr = errors.New("save Collection")
	fixture.handlers.Save(httptest.NewRecorder(), collectionWebRequest(http.MethodPost, "/collection/Shelf/0011223344556677", "included=true", "Shelf", "0011223344556677"))
	if fixture.failureStatus != http.StatusInternalServerError || fixture.failureMessage != "save Collection" {
		t.Fatalf("save failure = %q, %d", fixture.failureMessage, fixture.failureStatus)
	}
}

type collectionWebFixture struct {
	handlers                                               CollectionWebHandlers
	items, members                                         []library.Item
	page                                                   CollectionWebPage
	renderErr                                              error
	setErr                                                 error
	renderCalls, failureCalls, notFoundCalls               int
	visibleItemCalls, createCalls, setCalls, snapshotCalls int
	deleteCalls                                            int
	failureMessage                                         string
	failureStatus                                          int
}

func newCollectionWebFixture() *collectionWebFixture {
	fixture := &collectionWebFixture{
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
	fixture.handlers = NewCollectionWebHandlers(CollectionWebHandlersConfig{
		VisibleLibrary: func(*http.Request) ([]library.Item, error) { return fixture.items, nil },
		Snapshot: func(*http.Request) ([]library.Item, error) {
			fixture.snapshotCalls++
			return fixture.items, nil
		},
		VisibleItem: func(_ *http.Request, id string) (library.Item, bool) {
			fixture.visibleItemCalls++
			for _, item := range fixture.items {
				if item.ID == id {
					return item, true
				}
			}
			return library.Item{}, false
		},
		CollectionNames: func([]library.Item) []string { return []string{"Shelf"} },
		Collection:      func(string, []library.Item) []library.Item { return append([]library.Item(nil), fixture.members...) },
		Create: func(context.Context, string) error {
			fixture.createCalls++
			return nil
		},
		Set: func(context.Context, string, library.Item, bool) error {
			fixture.setCalls++
			return fixture.setErr
		},
		Delete: func(context.Context, string, []library.Item) error {
			fixture.deleteCalls++
			return nil
		},
		Owner: func(*http.Request) bool { return true },
		Render: func(_ http.ResponseWriter, _ *http.Request, page CollectionWebPage) error {
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

func collectionWebRequest(method, target, body, name, id string) *http.Request {
	request := httptest.NewRequestWithContext(context.Background(), method, target, strings.NewReader(body))
	if method == http.MethodPost {
		request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	if name != "" {
		request.SetPathValue("name", name)
	}
	if id != "" {
		request.SetPathValue("id", id)
	}
	return request
}
