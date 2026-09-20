package server

import (
	"errors"

	"github.com/MikeO7/kinosail/packages/credentials"
)

func (store *profileStore) resetPassword(id, password string) error {
	if err := credentials.Validate(password); err != nil {
		return err
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	profiles := cloneProfiles(store.profiles)
	for index := range profiles {
		if profiles[index].ID == id {
			if profiles[index].SCIMManaged {
				return errors.New("SCIM-managed profiles do not support local passwords")
			}
			credential, err := credentials.Hash(password)
			if err != nil {
				return err
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
	}
	return errors.New("viewer profile was not found")
}
