package playback

import (
	"errors"
	"sync"
	"time"
)

const maximumJellyfinSessions = 1024

// Serialize the bounded count and insert across the app-owned session maps.
var jellyfinSessionWrites sync.Mutex

func StoreJellyfinPlaySession(store *sync.Map, now time.Time, newID func() (string, error), session JellyfinSession) (string, error) { //nolint:cyclop // Validation, pruning, and insertion form one bounded operation.
	if store == nil || newID == nil || session == nil || now.IsZero() || !session.JellyfinExpires().After(now) {
		return "", errors.New("jellyfin play session input is invalid")
	}
	jellyfinSessionWrites.Lock()
	defer jellyfinSessionWrites.Unlock()
	count := 0
	store.Range(func(key, value any) bool {
		stored, valid := value.(JellyfinSession)
		if !valid || !stored.JellyfinExpires().After(now) {
			store.Delete(key)
		} else {
			count++
		}
		return true
	})
	if count >= maximumJellyfinSessions {
		return "", errors.New("too many active Jellyfin play sessions")
	}
	id, err := newID()
	if err != nil || id == "" || len(id) > 256 {
		if err == nil {
			err = errors.New("jellyfin play session ID is invalid")
		}
		return "", err
	}
	store.Store(id, session)
	return id, nil
}

// LoadJellyfinPlaySession removes expired entries before exposing them to delivery.
func LoadJellyfinPlaySession(store *sync.Map, id string, now time.Time) (any, bool) {
	value, found := store.Load(id)
	session, valid := value.(JellyfinSession)
	if !found || !valid || !now.Before(session.JellyfinExpires()) {
		store.Delete(id)
		return nil, false
	}
	return value, true
}
