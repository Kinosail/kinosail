package identitycore

import (
	"crypto/rand"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/MikeO7/kinosail/packages/credentials"
	"github.com/MikeO7/kinosail/packages/federation"
)

// ProfileStoreConfig binds Player's profile transactions to app-owned state.
type ProfileStoreConfig struct {
	Mutex       *sync.RWMutex
	Profiles    *[]Profile
	Sessions    *map[string]Session
	APIKeys     *map[string]APIKey
	Persistence ProfilePersistence
	Create      func(string, string, bool) (Profile, error)
	Now         func() time.Time
	NewID       func() string
}

// ProfileStore owns Player's canonical profile transaction policy.
type ProfileStore struct{ config ProfileStoreConfig }

// NewProfileStore returns the shared profile transaction module.
func NewProfileStore(config ProfileStoreConfig) ProfileStore {
	if config.Create == nil {
		config.Create = NewProfile
	}
	if config.Now == nil {
		config.Now = time.Now
	}
	if config.NewID == nil {
		config.NewID = rand.Text
	}
	return ProfileStore{config: config}
}

// HasProfiles reports whether setup has created an owner.
func (store ProfileStore) HasProfiles() bool {
	store.config.Mutex.RLock()
	defer store.config.Mutex.RUnlock()
	return len(*store.config.Profiles) > 0
}

// Authenticate verifies one active profile credential.
func (store ProfileStore) Authenticate(name, password string) (Profile, bool) {
	store.config.Mutex.RLock()
	defer store.config.Mutex.RUnlock()
	name = strings.TrimSpace(name)
	for _, profile := range *store.config.Profiles {
		if strings.EqualFold(profile.Name, name) && !profile.Disabled && !profile.SCIMDeleted {
			return profile, credentials.Verify(profile.Credential, password)
		}
	}
	credentials.DummyVerify(password)
	return Profile{}, false
}

// AddOwner persists a prebuilt first owner during app setup.
func (store ProfileStore) AddOwner(profile Profile) error {
	store.config.Mutex.Lock()
	defer store.config.Mutex.Unlock()
	if len(*store.config.Profiles) > 0 {
		return errors.New("owner profile already exists")
	}
	profiles := []Profile{profile}
	if err := store.config.Persistence.Save(profiles); err != nil {
		return err
	}
	*store.config.Profiles = profiles
	return nil
}

// AddProfile validates, persists, and commits one profile.
func (store ProfileStore) AddProfile(name, password string, owner bool, policy ProfilePolicy) (string, error) {
	if err := policy.Valid(); err != nil {
		return "", err
	}
	profile, err := store.config.Create(name, password, owner)
	if err != nil {
		return "", err
	}
	ApplyProfilePolicy(&profile, policy)
	store.config.Mutex.Lock()
	defer store.config.Mutex.Unlock()
	for _, existing := range *store.config.Profiles {
		if strings.EqualFold(existing.Name, profile.Name) {
			return "", errors.New("viewer profile name is already used")
		}
	}
	profiles := append(CloneProfiles(*store.config.Profiles), profile)
	if err := store.config.Persistence.Save(profiles); err != nil {
		return "", err
	}
	*store.config.Profiles = profiles
	return profile.ID, nil
}

// SetProfile atomically applies Player's owner and access policy.
func (store ProfileStore) SetProfile(id string, owner bool, policy ProfilePolicy) error {
	if err := policy.Valid(); err != nil {
		return err
	}
	store.config.Mutex.Lock()
	defer store.config.Mutex.Unlock()
	profiles := CloneProfiles(*store.config.Profiles)
	owners := 0
	for _, profile := range profiles {
		if profile.Owner {
			owners++
		}
	}
	for index := range profiles {
		if profiles[index].ID == id {
			return store.commitProfilePolicy(profiles, index, owner, owners, policy)
		}
	}
	return errors.New("viewer profile was not found")
}

func (store ProfileStore) commitProfilePolicy(profiles []Profile, index int, owner bool, owners int, policy ProfilePolicy) error {
	if profiles[index].Owner && !owner && owners == 1 {
		return errors.New("at least one Owner Profile is required")
	}
	profiles[index].Owner = owner
	ApplyProfilePolicy(&profiles[index], policy)
	profiles[index].Revision++
	if !owner && policy.Remote {
		if err := store.config.Persistence.Save(profiles); err != nil {
			return err
		}
		*store.config.Profiles = profiles
		return nil
	}
	sessions := WithoutProfile(*store.config.Sessions, profiles[index].ID, true)
	if err := store.config.Persistence.SaveRelated(profiles, sessions, *store.config.APIKeys); err != nil {
		return err
	}
	*store.config.Profiles, *store.config.Sessions = profiles, sessions
	return nil
}

// RemoveProfile atomically removes a local profile and its credentials.
func (store ProfileStore) RemoveProfile(id string) error { //nolint:cyclop,gocognit // Scores of 12 and 18 remain below the repository ceiling of 22 for one atomic removal transaction.
	store.config.Mutex.Lock()
	defer store.config.Mutex.Unlock()
	profiles := make([]Profile, 0, len(*store.config.Profiles))
	owners := 0
	for _, profile := range *store.config.Profiles {
		if profile.Owner {
			owners++
		}
	}
	removedID := ""
	for _, profile := range *store.config.Profiles {
		if profile.ID == id {
			if profile.SCIMManaged {
				return errors.New("SCIM-managed profiles are controlled by SCIM provisioning")
			}
			if profile.Owner && owners == 1 {
				return errors.New("at least one Owner Profile is required")
			}
			removedID = profile.ID
			continue
		}
		profiles = append(profiles, profile)
	}
	if removedID == "" {
		return errors.New("profile was not found")
	}
	sessions := WithoutProfile(*store.config.Sessions, removedID, false)
	keys := CloneAPIKeys(*store.config.APIKeys)
	for id, key := range keys {
		if key.ProfileID == removedID {
			delete(keys, id)
		}
	}
	if err := store.config.Persistence.SaveRelated(profiles, sessions, keys); err != nil {
		return err
	}
	*store.config.Profiles, *store.config.Sessions, *store.config.APIKeys = profiles, sessions, keys
	return nil
}

// ByID returns a detached profile by identifier.
func (store ProfileStore) ByID(id string) (Profile, bool) {
	store.config.Mutex.RLock()
	defer store.config.Mutex.RUnlock()
	return FindProfile(*store.config.Profiles, id)
}

// List returns a detached profile snapshot.
func (store ProfileStore) List() []Profile {
	store.config.Mutex.RLock()
	defer store.config.Mutex.RUnlock()
	return CloneProfiles(*store.config.Profiles)
}

// FederatedProfiles adapts profile state to the shared federation engine.
func (store ProfileStore) FederatedProfiles() federation.Profiles[Profile] {
	return federation.Profiles[Profile]{
		Lock: store.config.Mutex.Lock, Unlock: store.config.Mutex.Unlock,
		Clone:   func() []Profile { return CloneProfiles(*store.config.Profiles) },
		Persist: store.config.Persistence.Save,
		Commit:  func(profiles []Profile) { *store.config.Profiles = profiles },
		Inspect: FederatedProfile, Apply: ApplyFederatedIdentity,
	}
}
