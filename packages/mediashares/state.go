package mediashares

import (
	"encoding/base64"
	"encoding/hex"
)

// CloneState copies all nested state before a durable mutation.
func CloneState(value State) State {
	copy := State{Shares: make(map[string]Share, len(value.Shares)), Sessions: make(map[string]Session, len(value.Sessions))}
	for id, share := range value.Shares {
		share.ItemIDs = append([]string(nil), share.ItemIDs...)
		copy.Shares[id] = share
	}
	for id, session := range value.Sessions {
		copy.Sessions[id] = session
	}
	return copy
}

// RevokeAll removes all grants and active session accounting.
func (store *Store) RevokeAll() error {
	store.mu.Lock()
	defer store.mu.Unlock()
	if store.err != nil {
		return store.err
	}
	state := State{Shares: make(map[string]Share), Sessions: make(map[string]Session)}
	if err := store.persist(store.file, state); err != nil {
		return err
	}
	store.state, store.active = state, make(map[string]int)
	return nil
}

// Prune removes expired grants and orphaned sessions.
func Prune(state *State, now int64) {
	for id, share := range state.Shares {
		if share.ExpiresAt <= now {
			delete(state.Shares, id)
		}
	}
	for key, session := range state.Sessions {
		if session.ExpiresAt <= now || state.Shares[session.ShareID].ID == "" {
			delete(state.Sessions, key)
		}
	}
}

// ValidState checks every persisted grant and session boundary.
func ValidState(state State) bool {
	if len(state.Shares) > 256 || len(state.Sessions) > 2048 {
		return false
	}
	for id, share := range state.Shares {
		if !validShare(id, share) {
			return false
		}
	}
	for key, session := range state.Sessions {
		if !validSession(state.Shares, key, session) {
			return false
		}
	}
	return true
}

func validShare(id string, share Share) bool {
	if !validShareFields(id, share) {
		return false
	}
	return validItemIDs(share.ItemIDs)
}

func validShareFields(id string, share Share) bool {
	return id == share.ID && ValidSecret(id) && ValidHash(share.ClaimHash) && share.ExpiresAt > 0 && share.MaxDevices >= 1 && share.MaxDevices <= 8 && share.RightsAcknowledged
}

func validItemIDs(itemIDs []string) bool {
	if len(itemIDs) == 0 || len(itemIDs) > 100 {
		return false
	}
	seen := make(map[string]bool, len(itemIDs))
	for _, itemID := range itemIDs {
		if itemID == "" || len(itemID) > 256 || seen[itemID] {
			return false
		}
		seen[itemID] = true
	}
	return true
}

func validSession(shares map[string]Share, key string, session Session) bool {
	share, found := shares[session.ShareID]
	return ValidHash(key) && found && session.ExpiresAt > 0 && session.ExpiresAt <= share.ExpiresAt && len(session.Device) <= 80
}

// ValidSecret checks the public grant token encoding.
func ValidSecret(value string) bool {
	decoded, err := base64.RawURLEncoding.DecodeString(value)
	return err == nil && len(decoded) == 32
}

// ValidHash checks a persisted SHA-256 capability hash.
func ValidHash(value string) bool {
	decoded, err := hex.DecodeString(value)
	return err == nil && len(decoded) == 32
}
