package server

import (
	sharedpasskeys "github.com/MikeO7/kinosail/packages/passkeys"
	"github.com/go-webauthn/webauthn/webauthn"
)

func (store *profileStore) passkeyStore() *sharedpasskeys.ProfileStore[viewerProfile, viewerSession, apiKey] {
	return sharedpasskeys.NewProfileStore(sharedpasskeys.ProfileStoreConfig[viewerProfile, viewerSession, apiKey]{
		Mutex: &store.mu, Profiles: &store.profiles, Sessions: &store.sessions, Keys: &store.apiKeys,
		Access: sharedpasskeys.ProfileAccess[viewerProfile]{
			ID: func(profile *viewerProfile) string { return profile.ID }, Name: func(profile *viewerProfile) string { return profile.Name },
			Credentials: func(profile *viewerProfile) *[]webauthn.Credential { return &profile.Passkeys }, Usage: func(profile *viewerProfile) *map[string]passkeyUsage { return &profile.PasskeyUsage },
			Owner: func(profile *viewerProfile) bool { return profile.Owner }, AlternativeFactor: func(profile *viewerProfile) bool { return profile.TOTPSecret != "" },
			IncrementRevision: func(profile *viewerProfile) { profile.Revision++ }, Clone: cloneProfiles,
		},
		SaveProfiles: store.save, SaveRelated: store.saveRelated, WithoutPublicSessions: withoutPublicProfileSessions,
	})
}

func (store *profileStore) discoverPasskey(rawID, userHandle []byte) (webauthn.User, error) {
	return store.passkeyStore().Discover(rawID, userHandle)
}

func (store *profileStore) addPasskey(profileID string, credential *webauthn.Credential) error {
	return store.passkeyStore().Add(profileID, credential)
}

func (store *profileStore) updatePasskey(profileID string, credential *webauthn.Credential) error {
	return store.passkeyStore().Update(profileID, credential)
}

func (store *profileStore) passkeyInventory(profileID string) []passkeySummary {
	return store.passkeyStore().Inventory(profileID)
}

func (store *profileStore) removePasskey(profileID, id string) error {
	return store.passkeyStore().Remove(profileID, id)
}
