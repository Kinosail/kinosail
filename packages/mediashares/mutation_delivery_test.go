package mediashares

import (
	"strings"
	"testing"
	"time"

	"github.com/MikeO7/kinosail/packages/library"
)

func TestStoreDeliveryChecksEveryCapabilityState(t *testing.T) {
	now := int64(10)
	token, shareID := "session-token", indexedSecret(0)
	base := func() *Store {
		return &Store{
			state: State{
				Shares:   map[string]Share{shareID: {ID: shareID, ItemIDs: []string{"item"}, ExpiresAt: now + 1, MaxDevices: 1}},
				Sessions: map[string]Session{sessionKey(token): {ShareID: shareID, ExpiresAt: now + 1}},
			},
			find:   func(id string) (library.Item, bool) { return library.Item{ID: id, Path: "movie"}, id == "item" },
			active: make(map[string]map[string]int), now: func() time.Time { return time.Unix(now, 0) },
		}
	}
	request := capabilityRequest(t, token)
	for name, mutate := range map[string]func(*Store){
		"missing session": func(store *Store) { store.state.Sessions = map[string]Session{} },
		"missing share":   func(store *Store) { store.state.Shares = map[string]Share{} },
		"missing item":    func(store *Store) { store.find = func(string) (library.Item, bool) { return library.Item{}, false } },
		"expired session": func(store *Store) {
			store.state.Sessions[sessionKey(token)] = Session{ShareID: shareID, ExpiresAt: now}
		},
		"expired share": func(store *Store) {
			store.state.Shares[shareID] = Share{ID: shareID, ItemIDs: []string{"item"}, ExpiresAt: now, MaxDevices: 1}
		},
		"unlisted item": func(store *Store) {
			store.state.Shares[shareID] = Share{ID: shareID, ItemIDs: []string{"other"}, ExpiresAt: now + 1, MaxDevices: 1}
		},
		"another device at capacity": func(store *Store) { store.active[shareID] = map[string]int{indexedHash(1): 1} },
	} {
		store := base()
		mutate(store)
		if _, _, ok := store.Item(request, "item"); ok {
			t.Errorf("%s item was authorized", name)
		}
	}
	for name, mutate := range map[string]func(*Store){
		"missing session": func(store *Store) { store.state.Sessions = map[string]Session{} },
		"missing share":   func(store *Store) { store.state.Shares = map[string]Share{} },
		"expired session": func(store *Store) {
			store.state.Sessions[sessionKey(token)] = Session{ShareID: shareID, ExpiresAt: now}
		},
		"expired share": func(store *Store) {
			store.state.Shares[shareID] = Share{ID: shareID, ItemIDs: []string{"item"}, ExpiresAt: now, MaxDevices: 1}
		},
	} {
		store := base()
		mutate(store)
		if _, ok := store.Items(request); ok {
			t.Errorf("%s items were authorized", name)
		}
	}
	store := base()
	_, release, ok := store.Item(request, "item")
	if !ok {
		t.Fatal("stream was not admitted")
	}
	release()
	release()
	if _, again, ok := store.Item(request, "item"); !ok || store.active[shareID][sessionKey(token)] != 1 {
		t.Fatalf("released slot was not reusable: active=%v", store.active[shareID])
	} else {
		again()
	}
	if len(store.active) != 0 {
		t.Fatalf("release left active slots = %v", store.active)
	}
}

func TestRangeAndDeviceExactBoundaries(t *testing.T) {
	exactRange := "bytes=" + strings.Repeat("1", 121) + "-"
	if len(exactRange) != 128 || !ValidSingleByteRange(exactRange) {
		t.Fatal("maximum byte range was rejected")
	}
	device := strings.Repeat("x", 80)
	if cleanDeviceName(device) != device || len(cleanDeviceName(device+"x")) != 80 {
		t.Fatal("device boundary was not preserved")
	}
}
