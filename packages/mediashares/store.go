// Package mediashares owns Player's expiring private-media capability contract.
package mediashares

import (
	"cmp"
	"crypto/rand"
	"crypto/subtle"
	"errors"
	"io"
	"path/filepath"
	"slices"
	"sync"
	"time"

	"github.com/MikeO7/kinosail/packages/library"
)

// Share is one persisted media grant.
type Share struct {
	ID                 string   `json:"id"`
	ItemIDs            []string `json:"itemIds"`
	ClaimHash          string   `json:"claimHash"`
	ExpiresAt          int64    `json:"expiresAt"`
	MaxDevices         int      `json:"maxDevices"`
	RightsAcknowledged bool     `json:"rightsAcknowledged"`
}

// Session is one claimed device capability.
type Session struct {
	ShareID   string `json:"shareId"`
	ExpiresAt int64  `json:"expiresAt"`
	Device    string `json:"device"`
}

// State is the validated persisted grant document.
type State struct {
	Shares   map[string]Share   `json:"shares"`
	Sessions map[string]Session `json:"sessions"`
}

// Dependencies adapt application persistence and the current library index.
type Dependencies struct {
	Load     func(string, any) (bool, error)
	Persist  func(string, any) error
	Find     func(string) (library.Item, bool)
	Snapshot func() ([]library.Item, error)
	Now      func() time.Time
	Random   io.Reader
}

// Store owns media share grants, claims, stream limits, and persistence.
type Store struct {
	mu       sync.Mutex
	file     string
	state    State
	persist  func(string, any) error
	find     func(string) (library.Item, bool)
	snapshot func() ([]library.Item, error)
	active   map[string]int
	now      func() time.Time
	random   io.Reader
	err      error
}

// ErrDeviceLimit indicates that every permitted device has claimed the grant.
var ErrDeviceLimit = errors.New("media share device limit reached")

// View is the safe owner/API projection of a grant.
type View struct {
	ID         string   `json:"id"`
	ItemIDs    []string `json:"itemIds"`
	ExpiresAt  int64    `json:"expiresAt"`
	MaxDevices int      `json:"maxDevices"`
}

// New restores the media-share state and fails closed on invalid dependencies or state.
func New(dataDir string, dependencies Dependencies) *Store {
	dependencies, dependencyErr := normalizedDependencies(dependencies)
	store := &Store{file: filepath.Join(dataDir, "media_shares.json"), persist: dependencies.Persist, find: dependencies.Find, snapshot: dependencies.Snapshot, active: make(map[string]int), now: dependencies.Now, random: dependencies.Random, err: dependencyErr}
	if dependencyErr != nil {
		return store
	}
	store.restore(dataDir, dependencies.Load)
	return store
}

func normalizedDependencies(dependencies Dependencies) (Dependencies, error) {
	if dependencies.Now == nil {
		dependencies.Now = time.Now
	}
	if dependencies.Random == nil {
		dependencies.Random = rand.Reader
	}
	if dependencies.Load == nil || dependencies.Persist == nil || dependencies.Find == nil || dependencies.Snapshot == nil {
		return dependencies, errors.New("media share dependencies are not configured")
	}
	return dependencies, nil
}

func (store *Store) restore(dataDir string, load func(string, any) (bool, error)) {
	if dataDir != "" {
		if found, err := load(store.file, &store.state); err != nil {
			store.err = err
		} else if found && !ValidState(store.state) {
			store.err = errors.New("media share state is invalid")
		}
	}
	if store.state.Shares == nil {
		store.state.Shares = make(map[string]Share)
	}
	if store.state.Sessions == nil {
		store.state.Sessions = make(map[string]Session)
	}
}

// Create validates and persists one expiring grant.
func (store *Store) Create(itemIDs []string, lifetime time.Duration, maxDevices int, rightsAcknowledged bool) (View, string, error) { //nolint:cyclop // Every grant policy check must pass before persistence.
	if store.err != nil {
		return View{}, "", store.err
	}
	if !validCreatePolicy(itemIDs, lifetime, maxDevices, rightsAcknowledged) {
		return View{}, "", errors.New("media share policy is invalid")
	}
	seen := make(map[string]bool, len(itemIDs))
	for _, id := range itemIDs {
		item, found := store.find(id)
		if !found || item.Path == "" || seen[id] {
			return View{}, "", errors.New("media share content is invalid")
		}
		seen[id] = true
	}
	claim, err := store.secret()
	if err != nil {
		return View{}, "", err
	}
	id, err := store.secret()
	if err != nil {
		return View{}, "", err
	}
	now := store.now()
	share := Share{ID: id, ItemIDs: append([]string(nil), itemIDs...), ClaimHash: sessionKey(claim), ExpiresAt: now.Add(lifetime).Unix(), MaxDevices: maxDevices, RightsAcknowledged: true}
	store.mu.Lock()
	defer store.mu.Unlock()
	state := CloneState(store.state)
	Prune(&state, now.Unix())
	if len(state.Shares) >= 256 {
		return View{}, "", errors.New("too many active media shares")
	}
	state.Shares[share.ID] = share
	if err := store.persist(store.file, state); err != nil {
		return View{}, "", err
	}
	store.state = state
	return View{ID: share.ID, ItemIDs: append([]string(nil), share.ItemIDs...), ExpiresAt: share.ExpiresAt, MaxDevices: share.MaxDevices}, claim, nil
}

// Claim exchanges a one-time grant secret for a bounded device session.
func (store *Store) Claim(token, device string) (string, time.Time, error) { //nolint:cyclop // Token, expiry, device capacity, and atomic persistence form one claim decision.
	if !validClaimInput(token, device) {
		return "", time.Time{}, errors.New("media share claim is invalid")
	}
	now := store.now()
	store.mu.Lock()
	defer store.mu.Unlock()
	if store.err != nil {
		return "", time.Time{}, store.err
	}
	state := CloneState(store.state)
	var selected Share
	for _, share := range state.Shares {
		if subtle.ConstantTimeCompare([]byte(share.ClaimHash), []byte(sessionKey(token))) == 1 && share.ExpiresAt > now.Unix() {
			selected = share
		}
	}
	if selected.ID == "" {
		return "", time.Time{}, errors.New("media share was not found")
	}
	Prune(&state, now.Unix())
	devices := 0
	for _, session := range state.Sessions {
		if session.ShareID == selected.ID {
			devices++
		}
	}
	if devices >= selected.MaxDevices {
		return "", time.Time{}, ErrDeviceLimit
	}
	sessionToken, err := store.secret()
	if err != nil {
		return "", time.Time{}, err
	}
	expires := time.Unix(selected.ExpiresAt, 0)
	if maximum := now.Add(8 * time.Hour); maximum.Before(expires) {
		expires = maximum
	}
	state.Sessions[sessionKey(sessionToken)] = Session{ShareID: selected.ID, ExpiresAt: expires.Unix(), Device: cleanDeviceName(device)}
	if err := store.persist(store.file, state); err != nil {
		return "", time.Time{}, err
	}
	store.state = state
	return sessionToken, expires, nil
}

// Revoke removes a grant and all of its sessions.
func (store *Store) Revoke(id string) error {
	if !validCapabilityID(id) {
		return errors.New("media share was not found")
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	if store.err != nil {
		return store.err
	}
	if _, found := store.state.Shares[id]; !found {
		return errors.New("media share was not found")
	}
	state := CloneState(store.state)
	delete(state.Shares, id)
	for key, session := range state.Sessions {
		if session.ShareID == id {
			delete(state.Sessions, key)
		}
	}
	if err := store.persist(store.file, state); err != nil {
		return err
	}
	store.state = state
	return nil
}

func validCreatePolicy(itemIDs []string, lifetime time.Duration, maxDevices int, rightsAcknowledged bool) bool {
	return rightsAcknowledged && len(itemIDs) >= 1 && len(itemIDs) <= 100 && lifetime >= time.Minute && lifetime <= 24*time.Hour && maxDevices >= 1 && maxDevices <= 8
}

func validClaimInput(token, device string) bool {
	return validCapabilityID(token) && len(device) <= 80
}

func validCapabilityID(value string) bool {
	return len(value) >= 32 && len(value) <= 256
}

// List returns unexpired grants in expiry order.
func (store *Store) List() ([]View, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	if store.err != nil {
		return nil, store.err
	}
	result := make([]View, 0, len(store.state.Shares))
	for _, share := range store.state.Shares {
		if share.ExpiresAt > store.now().Unix() {
			result = append(result, View{ID: share.ID, ItemIDs: append([]string(nil), share.ItemIDs...), ExpiresAt: share.ExpiresAt, MaxDevices: share.MaxDevices})
		}
	}
	slices.SortFunc(result, func(left, right View) int { return cmp.Compare(left.ExpiresAt, right.ExpiresAt) })
	return result, nil
}
