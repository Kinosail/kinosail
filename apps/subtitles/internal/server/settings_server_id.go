package server

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
)

func (store *settingsStore) ensureServerID() error {
	if store.value.JellyfinID != "" {
		if !validServerID(store.value.JellyfinID) {
			return errors.New("server identifier is invalid")
		}
		return nil
	}
	data := make([]byte, 16)
	if _, err := rand.Read(data); err != nil {
		return err
	}
	store.value.JellyfinID = hex.EncodeToString(data)
	return store.save(store.value)
}

func validServerID(id string) bool {
	if len(id) > 128 {
		return false
	}
	for _, character := range id {
		if character != '-' && (character < '0' || character > '9') && (character < 'A' || character > 'Z') && (character < 'a' || character > 'z') {
			return false
		}
	}
	return true
}
