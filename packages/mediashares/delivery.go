package mediashares

import (
	"net/http"
	"slices"
	"sync"

	"github.com/MikeO7/kinosail/packages/identitycore"
	"github.com/MikeO7/kinosail/packages/library"
)

const maxStreamsPerDevice = 4

func (store *Store) Item(request *http.Request, id string) (string, func(), bool) {
	cookie, _ := request.Cookie("__Host-kinosail_share")
	if cookie == nil {
		return "", nil, false
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	if store.err != nil {
		return "", nil, false
	}
	device := sessionKey(cookie.Value)
	session, found := store.state.Sessions[device]
	share, active := store.state.Shares[session.ShareID]
	item, exists := store.find(id)
	if !canStream(found, active, exists, session, share, store.now().Unix(), slices.Contains(share.ItemIDs, id), store.active[share.ID], device) {
		return "", nil, false
	}
	if store.active[share.ID] == nil {
		store.active[share.ID] = make(map[string]int, share.MaxDevices)
	}
	store.active[share.ID][device]++
	return item.Path, sync.OnceFunc(func() { store.release(share.ID, device) }), true
}

func (store *Store) release(shareID, device string) {
	store.mu.Lock()
	defer store.mu.Unlock()
	devices := store.active[shareID]
	if devices[device] > 1 {
		devices[device]--
		return
	}
	delete(devices, device)
	if len(devices) == 0 {
		delete(store.active, shareID)
	}
}

func (store *Store) Items(request *http.Request) ([]library.Item, bool) {
	cookie, _ := request.Cookie("__Host-kinosail_share")
	if cookie == nil {
		return nil, false
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	if store.err != nil {
		return nil, false
	}
	session, found := store.state.Sessions[sessionKey(cookie.Value)]
	share, active := store.state.Shares[session.ShareID]
	if !canRead(found, active, session, share, store.now().Unix()) {
		return nil, false
	}
	items := make([]library.Item, 0, len(share.ItemIDs))
	for _, id := range share.ItemIDs {
		if item, exists := store.find(id); exists {
			items = append(items, item)
		}
	}
	return items, len(items) != 0
}

func canStream(found, active, exists bool, session Session, share Share, now int64, contains bool, streaming map[string]int, device string) bool {
	return found && active && exists && session.ExpiresAt > now && share.ExpiresAt > now && contains && hasStreamSlot(streaming, device, share.MaxDevices)
}

func hasStreamSlot(streaming map[string]int, device string, maxDevices int) bool {
	if streams, streamingNow := streaming[device]; streamingNow {
		return streams < maxStreamsPerDevice
	}
	return len(streaming) < maxDevices
}

func canRead(found, active bool, session Session, share Share, now int64) bool {
	return found && active && session.ExpiresAt > now && share.ExpiresAt > now
}

func (store *Store) Serve(writer http.ResponseWriter, request *http.Request) {
	if value := request.Header.Get("Range"); value != "" && !ValidSingleByteRange(value) {
		http.Error(writer, "invalid media range", http.StatusRequestedRangeNotSatisfiable)
		return
	}
	path, release, ok := store.Item(request, request.PathValue("id"))
	if !ok {
		http.NotFound(writer, request)
		return
	}
	defer release()
	writer.Header().Set("Cache-Control", "no-store, private")
	http.ServeFile(writer, request, path)
}

func ValidSingleByteRange(value string) bool {
	return identitycore.ValidSingleByteRange(value)
}
