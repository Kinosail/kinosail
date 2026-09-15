package servertest

import (
	"bytes"
	"encoding/base64"
	"strings"
	"testing"
	"time"

	"github.com/MikeO7/kinosail/packages/mediashares"
)

func mediaSharePersistedStateRejectsEveryInvalidField(t *testing.T) { //nolint:funlen // Persisted capability state must validate every field before use.
	secret := base64.RawURLEncoding.EncodeToString(make([]byte, 32))
	hash := strings.Repeat("0", 64)
	now := time.Now().Add(time.Hour).Unix()
	valid := func() mediashares.State {
		return mediashares.State{
			Shares:   map[string]mediashares.Share{secret: {ID: secret, ItemIDs: []string{"item"}, ClaimHash: hash, ExpiresAt: now, MaxDevices: 1, RightsAcknowledged: true}},
			Sessions: map[string]mediashares.Session{hash: {ShareID: secret, ExpiresAt: now, Device: "Browser"}},
		}
	}
	if !mediashares.ValidState(valid()) || !mediashares.ValidSecret(secret) || !mediashares.ValidHash(hash) {
		t.Fatal("valid persisted capability state was rejected")
	}
	tests := map[string]func(mediashares.State) mediashares.State{
		"share key mismatch": func(state mediashares.State) mediashares.State {
			share := state.Shares[secret]
			share.ID = "bad"
			state.Shares[secret] = share
			return state
		},
		"claim hash": func(state mediashares.State) mediashares.State {
			share := state.Shares[secret]
			share.ClaimHash = "bad"
			state.Shares[secret] = share
			return state
		},
		"share expiry": func(state mediashares.State) mediashares.State {
			share := state.Shares[secret]
			share.ExpiresAt = 0
			state.Shares[secret] = share
			return state
		},
		"device maximum": func(state mediashares.State) mediashares.State {
			share := state.Shares[secret]
			share.MaxDevices = 9
			state.Shares[secret] = share
			return state
		},
		"missing items": func(state mediashares.State) mediashares.State {
			share := state.Shares[secret]
			share.ItemIDs = nil
			state.Shares[secret] = share
			return state
		},
		"invalid item": func(state mediashares.State) mediashares.State {
			share := state.Shares[secret]
			share.ItemIDs = []string{""}
			state.Shares[secret] = share
			return state
		},
		"duplicate item": func(state mediashares.State) mediashares.State {
			share := state.Shares[secret]
			share.ItemIDs = []string{"item", "item"}
			state.Shares[secret] = share
			return state
		},
		"session hash": func(state mediashares.State) mediashares.State {
			state.Sessions["bad"] = state.Sessions[hash]
			delete(state.Sessions, hash)
			return state
		},
		"missing share": func(state mediashares.State) mediashares.State {
			session := state.Sessions[hash]
			session.ShareID = base64.RawURLEncoding.EncodeToString(bytes.Repeat([]byte{1}, 32))
			state.Sessions[hash] = session
			return state
		},
		"session expiry": func(state mediashares.State) mediashares.State {
			session := state.Sessions[hash]
			session.ExpiresAt = 0
			state.Sessions[hash] = session
			return state
		},
		"late session": func(state mediashares.State) mediashares.State {
			session := state.Sessions[hash]
			session.ExpiresAt = now + 1
			state.Sessions[hash] = session
			return state
		},
		"long device": func(state mediashares.State) mediashares.State {
			session := state.Sessions[hash]
			session.Device = strings.Repeat("x", 81)
			state.Sessions[hash] = session
			return state
		},
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			if mediashares.ValidState(mutate(valid())) {
				t.Fatal("invalid persisted capability state was accepted")
			}
		})
	}
}
