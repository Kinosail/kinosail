package mediashares

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/MikeO7/kinosail/packages/library"
)

func testMux(store *Store) *http.ServeMux {
	mux := http.NewServeMux()
	Register(mux, store, testViews(), func(next http.Handler) http.Handler { return next }, scriptHandler, executeTemplate, localError)
	return mux
}

func serveForm(t *testing.T, mux *http.ServeMux, path string, form url.Values) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, path, strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)
	return response
}

func TestWebHandlersCoverOwnerAndFailurePaths(t *testing.T) { //nolint:cyclop,funlen // One route matrix covers each web response branch.
	store, _ := testStore(t, time.Now(), library.Item{ID: "item", Title: "Movie", Path: "movie"})
	mux := testMux(store)
	owner := httptest.NewRecorder()
	mux.ServeHTTP(owner, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/settings/media-shares", nil))
	if owner.Code != http.StatusOK || !strings.Contains(owner.Body.String(), "Create a Media Share") {
		t.Fatalf("owner = %d %q", owner.Code, owner.Body.String())
	}
	invalid := serveForm(t, mux, "/settings/media-shares", url.Values{"itemIds": {"item"}})
	if invalid.Code != http.StatusBadRequest {
		t.Fatalf("invalid form = %d", invalid.Code)
	}
	unknown := serveForm(t, mux, "/settings/media-shares", url.Values{"itemIds": {"missing"}, "expires": {"3600"}, "devices": {"1"}, "rightsAcknowledged": {"true"}})
	if unknown.Code != http.StatusBadRequest {
		t.Fatalf("unknown content = %d", unknown.Code)
	}
	revoke := serveForm(t, mux, "/settings/media-shares/"+strings.Repeat("x", 32)+"/revoke", nil)
	if revoke.Code != http.StatusNotFound {
		t.Fatalf("unknown revoke = %d", revoke.Code)
	}

	failed := New("", Dependencies{})
	unavailable := httptest.NewRecorder()
	testMux(failed).ServeHTTP(unavailable, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/settings/media-shares", nil))
	if unavailable.Code != http.StatusServiceUnavailable {
		t.Fatalf("failed owner = %d", unavailable.Code)
	}
	listed := httptest.NewRecorder()
	failed.ListHTTP(listed, nil)
	if listed.Code != http.StatusServiceUnavailable {
		t.Fatalf("failed list = %d", listed.Code)
	}

	snapshotStore := New("", Dependencies{
		Load: func(string, any) (bool, error) { return false, nil }, Persist: func(string, any) error { return nil },
		Find:     func(string) (library.Item, bool) { return library.Item{}, false },
		Snapshot: func() ([]library.Item, error) { return nil, errors.New("snapshot failed") },
	})
	snapshot := httptest.NewRecorder()
	testMux(snapshotStore).ServeHTTP(snapshot, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/settings/media-shares", nil))
	if snapshot.Code != http.StatusServiceUnavailable {
		t.Fatalf("failed snapshot = %d", snapshot.Code)
	}
}

func TestAPIHandlersCoverMutationFailurePaths(t *testing.T) { //nolint:cyclop,funlen // One API matrix covers strict decoding and mapped failures.
	now := time.Now()
	store, state := testStore(t, now, library.Item{ID: "item", Path: "movie"})
	invalidClaim := httptest.NewRecorder()
	store.ClaimHTTP(invalidClaim, httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", strings.NewReader(`{`)))
	if invalidClaim.Code != http.StatusBadRequest {
		t.Fatalf("invalid claim = %d", invalidClaim.Code)
	}
	overflow := httptest.NewRecorder()
	store.CreateHTTP(overflow, httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", strings.NewReader(`{"itemIds":["item"],"expiresInSeconds":9223372036854775807,"maxDevices":1,"rightsAcknowledged":true}`)))
	if overflow.Code != http.StatusBadRequest || state.data != nil {
		t.Fatalf("overflow create = %d, persisted %q", overflow.Code, state.data)
	}
	share, claim, err := store.Create([]string{"item"}, time.Hour, 1, true)
	if err != nil {
		t.Fatal(err)
	}
	first := httptest.NewRecorder()
	store.ClaimHTTP(first, httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", strings.NewReader(`{"Token":"`+claim+`","Device":"Browser"}`)))
	limited := httptest.NewRecorder()
	store.ClaimHTTP(limited, httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", strings.NewReader(`{"Token":"`+claim+`","Device":"Second"}`)))
	if first.Code != http.StatusNoContent || limited.Code != http.StatusTooManyRequests {
		t.Fatalf("claims = %d, %d", first.Code, limited.Code)
	}
	revokeRequest := httptest.NewRequestWithContext(t.Context(), http.MethodDelete, "/", nil)
	revokeRequest.SetPathValue("id", share.ID)
	revoked := httptest.NewRecorder()
	store.RevokeHTTP(revoked, revokeRequest)
	if revoked.Code != http.StatusNoContent {
		t.Fatalf("revoke = %d", revoked.Code)
	}
}

func TestFormDecoderCoversMalformedAndPreparsedInputs(t *testing.T) {
	t.Parallel()
	malformed := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", strings.NewReader("itemIds=%"))
	malformed.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if _, _, _, err := decodeCreateForm(httptest.NewRecorder(), malformed); err == nil {
		t.Fatal("malformed form was accepted")
	}
	for _, expires := range []string{"invalid", "9223372036854775807"} {
		request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", nil)
		request.PostForm = url.Values{"itemIds": {"item"}, "expires": {expires}, "devices": {"1"}, "rightsAcknowledged": {"true"}}
		if _, _, _, err := decodeCreateForm(httptest.NewRecorder(), request); err == nil {
			t.Fatalf("expiry %q was accepted", expires)
		}
	}
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", nil)
	request.PostForm = url.Values{"itemIds": {"item"}, "expires": {"3600"}, "devices": {"invalid"}, "rightsAcknowledged": {"true"}}
	if _, _, _, err := decodeCreateForm(httptest.NewRecorder(), request); err == nil {
		t.Fatal("invalid device count was accepted")
	}
	request = httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", nil)
	request.ContentLength = 64<<10 + 1
	if _, _, _, err := decodeCreateForm(httptest.NewRecorder(), request); err == nil {
		t.Fatal("oversized form was accepted")
	}
}
