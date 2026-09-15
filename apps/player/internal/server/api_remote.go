package server

import "github.com/MikeO7/kinosail/packages/remoteaccess"

func revokePublicAuthorization(profiles *profileStore, passkeys *passkeyAuth, quick *quickConnectBroker, shares *mediaShareStore) error {
	var revokeShares func() error
	if shares != nil {
		revokeShares = shares.RevokeAll
	}
	return remoteaccess.RevokeAuthorization(passkeys.revokePublic, quick.revokeRemote, profiles.revokePublicSessions, revokeShares)
}
