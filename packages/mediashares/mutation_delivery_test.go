package mediashares

import (
	"strings"
	"testing"
	"time"

	"github.com/MikeO7/kinosail/packages/library"
)

func TestDeliveryAuthorizationChecksEveryCondition(t *testing.T) { //nolint:funlen // Each case isolates one capability condition.
	session := Session{ExpiresAt: 2}
	share := Share{ExpiresAt: 2, MaxDevices: 1}
	for name, test := range map[string]struct {
		found, active, exists, contains bool
		session                         Session
		share                           Share
		streams                         int
		valid                           bool
	}{
		"valid":           {true, true, true, true, session, share, 0, true},
		"missing session": {false, true, true, true, session, share, 0, false},
		"missing share":   {true, false, true, true, session, share, 0, false},
		"missing item":    {true, true, false, true, session, share, 0, false},
		"expired session": {true, true, true, true, Session{ExpiresAt: 1}, share, 0, false},
		"expired share":   {true, true, true, true, session, Share{ExpiresAt: 1, MaxDevices: 1}, 0, false},
		"unlisted item":   {true, true, true, false, session, share, 0, false},
		"at capacity":     {true, true, true, true, session, share, 1, false},
	} {
		actual := canStream(test.found, test.active, test.exists, test.session, test.share, 1, test.contains, test.streams)
		if actual != test.valid {
			t.Errorf("%s stream validity = %v", name, actual)
		}
	}
	for name, test := range map[string]struct {
		found, active bool
		session       Session
		share         Share
		valid         bool
	}{
		"valid": {true, true, session, share, true}, "missing session": {false, true, session, share, false},
		"missing share": {true, false, session, share, false}, "expired session": {true, true, Session{ExpiresAt: 1}, share, false},
		"expired share": {true, true, session, Share{ExpiresAt: 1}, false},
	} {
		if actual := canRead(test.found, test.active, test.session, test.share, 1); actual != test.valid {
			t.Errorf("%s read validity = %v", name, actual)
		}
	}
}

func TestStoreDeliveryChecksEveryCapabilityState(t *testing.T) { //nolint:cyclop,funlen,gocognit // Each case creates one complete capability state.
	now := int64(10)
	token, shareID := "session-token", indexedSecret(0)
	base := func() *Store {
		return &Store{
			state: State{
				Shares:   map[string]Share{shareID: {ID: shareID, ItemIDs: []string{"item"}, ExpiresAt: now + 1, MaxDevices: 1}},
				Sessions: map[string]Session{sessionKey(token): {ShareID: shareID, ExpiresAt: now + 1}},
			},
			find:   func(id string) (library.Item, bool) { return library.Item{ID: id, Path: "movie"}, id == "item" },
			active: make(map[string]int), now: func() time.Time { return time.Unix(now, 0) },
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
		"at capacity": func(store *Store) { store.active[shareID] = 1 },
	} {
		store := base()
		mutate(store)
		if _, ok := store.Item(request, "item"); ok {
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
	store.active[shareID] = 1
	store.Release(request)
	if store.active[shareID] != 0 {
		t.Fatal("release did not return the active slot")
	}
	store.Release(request)
	if store.active[shareID] != 0 {
		t.Fatal("release made the active count negative")
	}
	missing := base()
	missing.state.Sessions = map[string]Session{}
	missing.active[""] = 1
	missing.Release(request)
	if missing.active[""] != 1 {
		t.Fatal("missing session changed active slots")
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
