package server

import (
	"github.com/MikeO7/kinosail/packages/identitycore"
)

func (store *profileStore) byID(id string) (viewerProfile, bool) {
	return store.profileModule().ByID(id)
}

func (store *profileStore) list() []viewerProfile {
	return store.profileModule().List()
}

func cloneProfiles(profiles []viewerProfile) []viewerProfile {
	return identitycore.CloneProfiles(profiles)
}

func (store *profileStore) save(profiles []viewerProfile) error {
	return store.profilePersistence().Save(profiles)
}

func (store *profileStore) saveRelated(profiles []viewerProfile, sessions map[string]viewerSession, keys map[string]apiKey) error {
	return store.profilePersistence().SaveRelated(profiles, sessions, keys)
}

func (store *profileStore) profilePersistence() identitycore.ProfilePersistence {
	return identitycore.NewProfilePersistence(store.database, store.database != nil, store.persist, store.file, store.sessionFile, store.apiFile)
}

func (store *profileStore) profileModule() identitycore.ProfileStore {
	return identitycore.NewProfileStore(identitycore.ProfileStoreConfig{
		Mutex: &store.mu, Profiles: &store.profiles, Sessions: &store.sessions, APIKeys: &store.apiKeys,
		Persistence: store.profilePersistence(),
	})
}

var newProfile = identitycore.NewProfile
