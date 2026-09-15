package server

import (
	"net/http"

	"github.com/MikeO7/kinosail/packages/federation"
)

func (store *profileStore) federatedProfiles() federation.Profiles[viewerProfile] {
	return store.profileModule().FederatedProfiles()
}

func federationWebHooks(store *profileStore) federation.WebHooks[viewerProfile] {
	return federation.WebHooks[viewerProfile]{
		CurrentProfileID:   func(request *http.Request) string { return currentViewer(request).ID },
		SecureRequest:      secureRequest,
		ProfileID:          func(profile viewerProfile) string { return profile.ID },
		RequiresMFA:        func(profile viewerProfile) bool { return profile.TOTPSecret != "" },
		VerifySecondFactor: store.verifySecondFactor, SignIn: store.signIn,
		Audit: setAuditViewer, Error: localizedError, APIError: apiError,
	}
}
