package server

import (
	"net/http"
	"time"

	"github.com/MikeO7/kinosail/packages/federation"
)

func (store *profileStore) federatedProfiles() federation.Profiles[viewerProfile] {
	profiles := store.profileModule().FederatedProfiles()
	profiles.SessionActive = func(key, id string) bool {
		session, found := store.sessions[key]
		if !found || session.ProfileID != id || session.Channel != "" || store.sessionExpired(session, time.Now().Unix()) {
			return false
		}
		for _, profile := range store.profiles {
			if profile.ID == id {
				return session.ProfileRevision == profile.Revision && session.StrongAt != 0 && time.Unix(session.StrongAt, 0).Add(10*time.Minute).After(time.Now())
			}
		}
		return false
	}
	return profiles
}

func federationWebHooks(store *profileStore) federation.WebHooks[viewerProfile] {
	return federation.WebHooks[viewerProfile]{
		LinkSession:        func(request *http.Request) string { return sessionKey(sessionToken(request)) },
		CurrentProfileID:   func(request *http.Request) string { return currentViewer(request).ID },
		SecureRequest:      secureRequest,
		ProfileID:          func(profile viewerProfile) string { return profile.ID },
		RequiresMFA:        func(profile viewerProfile) bool { return profile.TOTPSecret != "" },
		VerifySecondFactor: store.verifySecondFactor, SignIn: store.signInStrong,
		Audit: setAuditViewer, Error: localizedError, APIError: apiError,
	}
}
