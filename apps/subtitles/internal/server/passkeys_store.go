package server

import (
	sharedpasskeys "github.com/MikeO7/kinosail/packages/passkeys"
	"github.com/go-webauthn/webauthn/webauthn"
)

func (store *profileStore) passkeyStore() *sharedpasskeys.ProfileStore[viewerProfile, viewerSession, apiKey] {
	access := sharedpasskeys.ProfileAccess[viewerProfile]{
		ID: subtitlePasskeyProfileID, Name: subtitlePasskeyProfileName,
		Credentials: subtitlePasskeyCredentials, Usage: subtitlePasskeyUsage,
		Owner: subtitlePasskeyOwner, AlternativeFactor: subtitlePasskeyAlternativeFactor,
		IncrementRevision: subtitlePasskeyIncrementRevision, Clone: cloneProfiles,
	}
	config := sharedpasskeys.ProfileStoreConfig[viewerProfile, viewerSession, apiKey]{
		Mutex: &store.mu, Profiles: &store.profiles, Sessions: &store.sessions, Keys: &store.apiKeys, Access: access,
		SaveProfiles: store.save, SaveRelated: store.saveRelated, WithoutPublicSessions: withoutPublicProfileSessions,
	}
	return sharedpasskeys.NewProfileStore(config)
}

func (store *profileStore) addPasskey(profileID string, credential *webauthn.Credential) error {
	return store.passkeyStore().Add(profileID, credential)
}

func (store *profileStore) discoverPasskey(rawID, userHandle []byte) (webauthn.User, error) {
	return store.passkeyStore().Discover(rawID, userHandle)
}

func (store *profileStore) passkeyInventory(profileID string) []passkeySummary {
	return store.passkeyStore().Inventory(profileID)
}

func (store *profileStore) removePasskey(profileID, id string) error {
	return store.passkeyStore().Remove(profileID, id)
}

func (store *profileStore) updatePasskey(profileID string, credential *webauthn.Credential) error {
	return store.passkeyStore().Update(profileID, credential)
}

func subtitlePasskeyProfileID(profile *viewerProfile) string   { return profile.ID }
func subtitlePasskeyProfileName(profile *viewerProfile) string { return profile.Name }
func subtitlePasskeyCredentials(profile *viewerProfile) *[]webauthn.Credential {
	return &profile.Passkeys
}

func subtitlePasskeyUsage(profile *viewerProfile) *map[string]passkeyUsage {
	return &profile.PasskeyUsage
}
func subtitlePasskeyOwner(profile *viewerProfile) bool             { return profile.Owner }
func subtitlePasskeyAlternativeFactor(profile *viewerProfile) bool { return profile.TOTPSecret != "" }
func subtitlePasskeyIncrementRevision(profile *viewerProfile)      { profile.Revision++ }
