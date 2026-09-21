package catalog

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/MikeO7/kinosail/packages/library"
)

func TestCollectionWebMutationsReportStorageFailures(t *testing.T) {
	t.Parallel()
	for _, operation := range []string{"create", "delete", "save", "manage"} {
		t.Run(operation, func(t *testing.T) { assertCollectionStorageFailure(t, operation) })
	}
}

func assertCollectionStorageFailure(t *testing.T, operation string) {
	t.Helper()
	fixture := newCollectionWebFixture()
	failure := errors.New("storage unavailable")
	fixture.setErr = failure
	fixture.handlers.config.Create = func(context.Context, string) error { return failure }
	fixture.handlers.config.Delete = func(context.Context, string, []library.Item) error { return failure }
	request := collectionWebRequest("POST", "/collection/Shelf", "name=Shelf&included=true", "Shelf", "0011223344556677")
	response := httptest.NewRecorder()
	switch operation {
	case "create":
		fixture.handlers.Create(response, request)
	case "delete":
		fixture.handlers.Delete(response, request)
	case "save":
		fixture.handlers.Save(response, request)
	case "manage":
		request = collectionWebRequest("POST", "/collection/Shelf/items/0011223344556677", "included=true", "Shelf", "0011223344556677")
		fixture.handlers.ManageItem(response, request)
	}
	if response.Code != http.StatusInternalServerError {
		t.Fatalf("%s status=%d", operation, response.Code)
	}
}

func TestCollectionMembershipDistinguishesMissingFromFailedStorage(t *testing.T) {
	t.Parallel()
	fixture := newCollectionWebFixture()
	fixture.setErr = os.ErrNotExist
	for _, handler := range []http.HandlerFunc{fixture.handlers.Save, fixture.handlers.ManageItem} {
		response := httptest.NewRecorder()
		handler(response, collectionWebRequest("POST", "/collection/Shelf/items/0011223344556677", "included=true", "Shelf", "0011223344556677"))
		if response.Code != 404 {
			t.Fatalf("missing collection status=%d", response.Code)
		}
	}
	fixture.setErr = nil
	response := httptest.NewRecorder()
	fixture.handlers.ManageItem(response, collectionWebRequest("POST", "/collection/Shelf/items/aaaaaaaaaaaaaaaa", "included=true", "Shelf", "aaaaaaaaaaaaaaaa"))
	if response.Code != 404 {
		t.Fatalf("invisible item status=%d", response.Code)
	}
}

func TestPlaylistMembershipFailureDoesNotRedirect(t *testing.T) {
	t.Parallel()
	for _, failure := range []error{os.ErrNotExist, errors.New("storage unavailable")} {
		fixture := newPlaylistHTTPFixture()
		fixture.handlers.config.SetPlaylist = func(context.Context, string, string, string, bool) error { return failure }
		response := httptest.NewRecorder()
		fixture.handlers.ManageItem(response, collectionWebRequest("POST", "/playlist/Favorites/items/0011223344556677", "included=true", "Favorites", "0011223344556677"))
		want := 500
		if errors.Is(failure, os.ErrNotExist) {
			want = 404
		}
		if response.Code != want || response.Header().Get("Location") != "" {
			t.Fatalf("membership=%d %s", response.Code, response.Header().Get("Location"))
		}
	}
}
