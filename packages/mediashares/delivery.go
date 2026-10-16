package mediashares

import (
	"net/http"
	"slices"
	"sync"

	"github.com/MikeO7/kinosail/packages/identitycore"
	"github.com/MikeO7/kinosail/packages/library"
)

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
	session, found := store.state.Sessions[sessionKey(cookie.Value)]
	share, active := store.state.Shares[session.ShareID]
	item, exists := store.find(id)
	if !canStream(found, active, exists, session, share, store.now().Unix(), slices.Contains(share.ItemIDs, id), store.active[share.ID]) {
		return "", nil, false
	}
	store.active[share.ID]++
	return item.Path, sync.OnceFunc(func() { store.release(share.ID) }), true
}

func (store *Store) release(shareID string) {
	store.mu.Lock()
	defer store.mu.Unlock()
	if store.active[shareID] > 1 {
		store.active[shareID]--
		return
	}
	delete(store.active, shareID)
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

func canStream(found, active, exists bool, session Session, share Share, now int64, contains bool, streams int) bool {
	return found && active && exists && session.ExpiresAt > now && share.ExpiresAt > now && contains && streams < share.MaxDevices
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
