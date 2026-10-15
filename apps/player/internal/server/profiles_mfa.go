package server

import (
	"github.com/MikeO7/kinosail/packages/identitycore"
)

func (store *profileStore) enableMFA(id, secret string, recovery []string) error {
	store.mu.Lock()
	defer store.mu.Unlock()
	return store.factorTransaction().Enable(id, secret, recovery)
}

func (store *profileStore) disableMFA(id string) error {
	store.mu.Lock()
	defer store.mu.Unlock()
	return store.factorTransaction().Disable(id)
}

func (store *profileStore) verifySecondFactor(id, code string) bool {
	store.mu.Lock()
	defer store.mu.Unlock()
	return store.factorTransaction().Verify(id, code)
}

func (store *profileStore) factorTransaction() *identitycore.FactorTransaction[viewerProfile] {
	return identitycore.NewFactorTransaction(identitycore.FactorConfig[viewerProfile]{
		Profiles: &store.profiles, Sessions: &store.sessions, Project: factorProfile, Apply: applyFactorProfile,
		Persist: func(profiles []viewerProfile, sessions map[string]viewerSession, related bool) error {
			if related {
				return store.saveRelated(profiles, sessions, store.apiKeys)
			}
			return store.save(profiles)
		},
	})
}

func factorProfile(profile viewerProfile) identitycore.FactorProfile {
	return identitycore.FactorProfile{ID: profile.ID, Owner: profile.Owner, Passkeys: len(profile.Passkeys), Secret: profile.TOTPSecret, Recovery: profile.Recovery, Counter: profile.TOTPCounter, Revision: profile.Revision}
}

func applyFactorProfile(profile viewerProfile, factor identitycore.FactorProfile) viewerProfile {
	profile.TOTPSecret, profile.Recovery, profile.TOTPCounter, profile.Revision = factor.Secret, factor.Recovery, factor.Counter, factor.Revision
	return profile
}
