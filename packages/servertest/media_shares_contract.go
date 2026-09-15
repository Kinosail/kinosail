package servertest

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/MikeO7/kinosail/packages/library"
	"github.com/MikeO7/kinosail/packages/mediashares"
)

// MediaShareFactory binds the real app's library index and persisted share store.
type MediaShareFactory func(string, []library.Item, bool) *mediashares.Store

// MediaShareContract checks grant delivery and persisted-state validation.
func MediaShareContract(t *testing.T, newStore MediaShareFactory) {
	t.Helper()
	t.Run("MediaShareCapabilityBoundaries", func(t *testing.T) { mediaShareCapabilityBoundaries(t, newStore) })
	t.Run("MediaSharePersistedStateRejectsEveryInvalidField", mediaSharePersistedStateRejectsEveryInvalidField)
	t.Run("MediaSharesFailClosedOnCorruptPersistedGrantState", func(t *testing.T) { mediaSharesFailClosedOnCorruptPersistedGrantState(t, newStore) })
}

func mediaShareCapabilityBoundaries(t *testing.T, newStore MediaShareFactory) {
	directory := t.TempDir()
	media := filepath.Join(directory, "movie.mp4")
	if err := os.WriteFile(media, []byte("movie"), 0o600); err != nil {
		t.Fatal(err)
	}
	store := newStore(directory, []library.Item{{ID: "item", Path: media}}, false)
	share, claim, err := store.Create([]string{"item"}, time.Hour, 1, true)
	if err != nil {
		t.Fatal(err)
	}
	if listed, listErr := store.List(); listErr != nil || len(listed) != 1 || listed[0].ID != share.ID {
		t.Fatalf("active shares = %#v, %v", listed, listErr)
	}
	request := assertMediaShareClaim(t, store, claim, media)
	store.Release(request)
	if _, ok := store.Item(request, "item"); !ok {
		t.Fatal("first active stream was rejected")
	}
	if _, ok := store.Item(request, "item"); ok {
		t.Fatal("active stream limit was bypassed")
	}
	store.Release(request)
	assertMediaShareRanges(t, store)
	if err := store.Revoke("short"); err == nil {
		t.Fatal("invalid share ID was revoked")
	}
}

func mediaSharesFailClosedOnCorruptPersistedGrantState(t *testing.T, newStore MediaShareFactory) {
	directory := t.TempDir()
	if err := os.WriteFile(filepath.Join(directory, "media_shares.json"), []byte(`{"shares":{"bad":{"id":"bad"}},"sessions":{}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	store := newStore(directory, []library.Item{{ID: "item", Path: "item.mp4"}}, false)
	if _, err := store.List(); err == nil {
		t.Fatal("corrupt persisted Media Share state was accepted")
	}
	if _, _, err := store.Create([]string{"item"}, time.Hour, 1, true); err == nil {
		t.Fatal("corrupt persisted state allowed a new grant")
	}
}

func assertMediaShareClaim(t *testing.T, store *mediashares.Store, claim, media string) *http.Request {
	t.Helper()
	withoutCookie := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/share/items", nil)
	if _, ok := store.Items(withoutCookie); ok {
		t.Fatal("missing capability listed shared items")
	}
	session, _, err := store.Claim(claim, "Guest browser")
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := store.Claim(claim, "Second browser"); !errors.Is(err, mediashares.ErrDeviceLimit) {
		t.Fatalf("second device claim = %v", err)
	}
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/share/items", nil)
	request.AddCookie(&http.Cookie{Name: "__Host-kinosail_share", Value: session, Path: "/", Secure: true, HttpOnly: true, SameSite: http.SameSiteStrictMode})
	if items, ok := store.Items(request); !ok || len(items) != 1 || items[0].ID != "item" {
		t.Fatalf("shared items = %#v, %v", items, ok)
	}
	path, ok := store.Item(request, "item")
	if !ok || path != media {
		t.Fatalf("shared item = %q, %v", path, ok)
	}
	return request
}

func assertMediaShareRanges(t *testing.T, store *mediashares.Store) {
	t.Helper()
	invalidRange := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/share/media/item", nil)
	invalidRange.Header.Set("Range", "bytes=0-1,3-4")
	invalidRange.SetPathValue("id", "item")
	response := httptest.NewRecorder()
	store.Serve(response, invalidRange)
	if response.Code != http.StatusRequestedRangeNotSatisfiable {
		t.Fatalf("invalid range = %d", response.Code)
	}
	for value, valid := range map[string]bool{"bytes=0-": true, "bytes=-10": true, "bytes=1-2": true, "": false, "items=0-1": false, strings.Repeat("x", 129): false} {
		if mediashares.ValidSingleByteRange(value) != valid {
			t.Errorf("range %q validity = %v", value, !valid)
		}
	}
}
