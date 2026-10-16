package server

import (
	"errors"

	"github.com/MikeO7/kinosail/packages/credentials"
)

func (store *profileStore) resetPassword(id, password string) error {
	if err := credentials.Validate(password); err != nil {
		return err
	}
	store.mu.RLock()
	current, err := passwordResetTarget(store.profiles, id)
	store.mu.RUnlock()
	if err != nil {
		return err
	}
	credential, err := credentials.Hash(password)
	if err != nil {
		return err
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	profiles := cloneProfiles(store.profiles)
	for index := range profiles {
		if profiles[index].ID != id {
			continue
		}
		if profiles[index].SCIMManaged {
			return errors.New("SCIM-managed profiles do not support local passwords")
		}
		if profiles[index].Revision != current.Revision {
			return errors.New("viewer profile changed during the password reset; try again")
		}
		profiles[index].Credential = credential
		profiles[index].Revision++
		sessions := withoutProfile(store.sessions, id)
		if err := store.saveRelated(profiles, sessions, store.apiKeys); err != nil {
			return err
		}
		store.profiles, store.sessions = profiles, sessions
		return nil
	}
	return errors.New("viewer profile was not found")
}

func passwordResetTarget(profiles []viewerProfile, id string) (viewerProfile, error) {
	for _, profile := range profiles {
		if profile.ID == id {
			if profile.SCIMManaged {
				return viewerProfile{}, errors.New("SCIM-managed profiles do not support local passwords")
			}
			return profile, nil
		}
	}
	return viewerProfile{}, errors.New("viewer profile was not found")
}
