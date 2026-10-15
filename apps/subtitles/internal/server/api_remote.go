package server

import "github.com/MikeO7/kinosail/packages/remoteaccess"

func revokePublicAuthorization(profiles *profileStore, passkeys *passkeyAuth) error {
	return remoteaccess.RevokeAuthorization(passkeys.revokePublic, func() {}, profiles.revokePublicSessions, nil)
}
