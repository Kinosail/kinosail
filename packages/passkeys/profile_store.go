package passkeys

import (
	"sync"
	"time"

	"github.com/go-webauthn/webauthn/webauthn"
)

// ProfileAccess maps one application profile to shared passkey state.
type ProfileAccess[T any] struct {
	ID                func(*T) string
	Name              func(*T) string
	Credentials       func(*T) *[]webauthn.Credential
	Usage             func(*T) *map[string]Usage
	Owner             func(*T) bool
	AlternativeFactor func(*T) bool
	IncrementRevision func(*T)
	Clone             func([]T) []T
}

// ProfileStoreConfig connects shared passkey rules to application persistence.
type ProfileStoreConfig[T, S, K any] struct {
	Mutex                 *sync.RWMutex
	Profiles              *[]T
	Sessions              *map[string]S
	Keys                  *map[string]K
	Access                ProfileAccess[T]
	SaveProfiles          func([]T) error
	SaveRelated           func([]T, map[string]S, map[string]K) error
	WithoutPublicSessions func(map[string]S, string) map[string]S
	Now                   func() time.Time
}

// ProfileStore owns shared profile passkey operations.
type ProfileStore[T, S, K any] struct{ config ProfileStoreConfig[T, S, K] }

// NewProfileStore creates one adapter-backed profile store.
func NewProfileStore[T, S, K any](config ProfileStoreConfig[T, S, K]) *ProfileStore[T, S, K] {
	if config.Now == nil {
		config.Now = time.Now
	}
	return &ProfileStore[T, S, K]{config: config}
}

// Discover returns an isolated profile for a discoverable credential.
func (store *ProfileStore[T, S, K]) Discover(rawID, userHandle []byte) (webauthn.User, error) {
	if err := ValidateLookup(rawID, userHandle); err != nil || !store.valid() {
		return nil, ErrInvalidCredential
	}
	store.config.Mutex.RLock()
	defer store.config.Mutex.RUnlock()
	for index := range *store.config.Profiles {
		profile := &(*store.config.Profiles)[index]
		if store.config.Access.ID(profile) == string(userHandle) && Index(*store.config.Access.Credentials(profile), rawID) >= 0 {
			clone := store.config.Access.Clone([]T{*profile})[0]
			return NewUser(clone, store.config.Access.ID(&clone), store.config.Access.Name(&clone), *store.config.Access.Credentials(&clone)), nil
		}
	}
	return nil, ErrNotFound
}

// Add validates and commits one unique credential.
func (store *ProfileStore[T, S, K]) Add(profileID string, credential *webauthn.Credential) error {
	if err := ValidateCredential(credential); err != nil || !store.valid() {
		return ErrInvalidCredential
	}
	store.config.Mutex.Lock()
	defer store.config.Mutex.Unlock()
	profiles := store.config.Access.Clone(*store.config.Profiles)
	for index := range profiles {
		if Index(*store.config.Access.Credentials(&profiles[index]), credential.ID) >= 0 {
			return ErrDuplicate
		}
	}
	for index := range profiles {
		profile := &profiles[index]
		if store.config.Access.ID(profile) != profileID {
			continue
		}
		credentials := store.config.Access.Credentials(profile)
		if len(*credentials) >= MaxCredentials {
			return ErrLimit
		}
		*credentials = append(*credentials, Clone(*credential))
		usage := store.config.Access.Usage(profile)
		if *usage == nil {
			*usage = make(map[string]Usage)
		}
		(*usage)[ID(credential.ID)] = Usage{Tracked: true}
		store.config.Access.IncrementRevision(profile)
		sessions := store.config.WithoutPublicSessions(*store.config.Sessions, profileID)
		if err := store.config.SaveRelated(profiles, sessions, *store.config.Keys); err != nil {
			return err
		}
		*store.config.Profiles, *store.config.Sessions = profiles, sessions
		return nil
	}
	return ErrNotFound
}

// Update validates and commits one verified credential counter.
func (store *ProfileStore[T, S, K]) Update(profileID string, credential *webauthn.Credential) error {
	if err := ValidateCredential(credential); err != nil || !store.valid() {
		return ErrInvalidCredential
	}
	store.config.Mutex.Lock()
	defer store.config.Mutex.Unlock()
	profiles := store.config.Access.Clone(*store.config.Profiles)
	for index := range profiles {
		profile := &profiles[index]
		if store.config.Access.ID(profile) != profileID {
			continue
		}
		usage := store.config.Access.Usage(profile)
		if *usage == nil {
			*usage = make(map[string]Usage)
		}
		if Update(*store.config.Access.Credentials(profile), *usage, credential, store.config.Now()) {
			if err := store.config.SaveProfiles(profiles); err != nil {
				return err
			}
			*store.config.Profiles = profiles
			return nil
		}
	}
	return ErrNotFound
}

// Inventory returns redacted passkey details for one profile.
func (store *ProfileStore[T, S, K]) Inventory(profileID string) []Summary {
	if !store.valid() {
		return []Summary{}
	}
	store.config.Mutex.RLock()
	defer store.config.Mutex.RUnlock()
	for index := range *store.config.Profiles {
		profile := &(*store.config.Profiles)[index]
		if store.config.Access.ID(profile) == profileID {
			return Inventory(*store.config.Access.Credentials(profile), *store.config.Access.Usage(profile))
		}
	}
	return []Summary{}
}

// Remove validates and commits one credential removal.
func (store *ProfileStore[T, S, K]) Remove(profileID, id string) error {
	if err := ValidateID(id); err != nil || !store.valid() {
		return ErrInvalidID
	}
	store.config.Mutex.Lock()
	defer store.config.Mutex.Unlock()
	profiles := store.config.Access.Clone(*store.config.Profiles)
	if err := store.remove(profiles, profileID, id); err != nil {
		return err
	}
	sessions := store.config.WithoutPublicSessions(*store.config.Sessions, profileID)
	if err := store.config.SaveRelated(profiles, sessions, *store.config.Keys); err != nil {
		return err
	}
	*store.config.Profiles, *store.config.Sessions = profiles, sessions
	return nil
}

func (store *ProfileStore[T, S, K]) valid() bool {
	if store == nil {
		return false
	}
	config := store.config
	return validProfileStoreState(config) && validProfileAccess(config.Access)
}

func (store *ProfileStore[T, S, K]) remove(profiles []T, profileID, id string) error {
	for index := range profiles {
		profile := &profiles[index]
		if store.config.Access.ID(profile) == profileID {
			return store.removeCredential(profile, id)
		}
	}
	return ErrNotFound
}

func (store *ProfileStore[T, S, K]) removeCredential(profile *T, id string) error {
	credentials := store.config.Access.Credentials(profile)
	for index := range *credentials {
		if ID((*credentials)[index].ID) != id {
			continue
		}
		if store.config.Access.Owner(profile) && len(*credentials) == 1 && !store.config.Access.AlternativeFactor(profile) {
			return ErrLastFactor
		}
		*credentials = append((*credentials)[:index], (*credentials)[index+1:]...)
		usage := store.config.Access.Usage(profile)
		delete(*usage, id)
		if len(*usage) == 0 {
			*usage = nil
		}
		store.config.Access.IncrementRevision(profile)
		return nil
	}
	return ErrNotFound
}

func validProfileStoreState[T, S, K any](config ProfileStoreConfig[T, S, K]) bool {
	return config.Mutex != nil && config.Profiles != nil && config.Sessions != nil && config.Keys != nil && config.SaveProfiles != nil && config.SaveRelated != nil && config.WithoutPublicSessions != nil
}

func validProfileAccess[T any](access ProfileAccess[T]) bool {
	return access.ID != nil && access.Name != nil && access.Credentials != nil && access.Usage != nil && access.Owner != nil && access.AlternativeFactor != nil && access.IncrementRevision != nil && access.Clone != nil
}
