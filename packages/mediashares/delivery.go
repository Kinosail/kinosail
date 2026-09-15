package mediashares

import (
	"net/http"
	"regexp"
	"slices"

	"github.com/MikeO7/kinosail/packages/library"
)

// Item reserves one stream slot for an authorized item.
func (store *Store) Item(request *http.Request, id string) (string, bool) {
	cookie, _ := request.Cookie("__Host-kinosail_share")
	if cookie == nil {
		return "", false
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	if store.err != nil {
		return "", false
	}
	session, found := store.state.Sessions[sessionKey(cookie.Value)]
	share, active := store.state.Shares[session.ShareID]
	item, exists := store.find(id)
	if !canStream(found, active, exists, session, share, store.now().Unix(), slices.Contains(share.ItemIDs, id), store.active[share.ID]) {
		return "", false
	}
	store.active[share.ID]++
	return item.Path, true
}

// Items returns the authorized library projection for a claimed session.
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

// Release returns one active stream slot.
func (store *Store) Release(request *http.Request) {
	cookie, _ := request.Cookie("__Host-kinosail_share")
	if cookie == nil {
		return
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	if session, found := store.state.Sessions[sessionKey(cookie.Value)]; found && store.active[session.ShareID] > 0 {
		store.active[session.ShareID]--
	}
}

// Serve delivers one authorized local media file with strict range syntax.
func (store *Store) Serve(writer http.ResponseWriter, request *http.Request) {
	if value := request.Header.Get("Range"); value != "" && !ValidSingleByteRange(value) {
		http.Error(writer, "invalid media range", http.StatusRequestedRangeNotSatisfiable)
		return
	}
	path, ok := store.Item(request, request.PathValue("id"))
	if !ok {
		http.NotFound(writer, request)
		return
	}
	defer store.Release(request)
	writer.Header().Set("Cache-Control", "no-store, private")
	http.ServeFile(writer, request, path)
}

var singleByteRange = regexp.MustCompile(`^bytes=(?:[0-9]+-[0-9]*|-[0-9]+)$`)

// ValidSingleByteRange accepts exactly one bounded byte range.
func ValidSingleByteRange(value string) bool {
	return len(value) <= 128 && singleByteRange.MatchString(value)
}
