package configuration

import (
	sharedvalidation "github.com/MikeO7/kinosail/packages/validation"
)

func validateRemoteAccess(configured Snapshot) error {
	return sharedvalidation.ValidateRemoteAccess(configured.String)
}

func validRemoteToken(token string) bool { return sharedvalidation.ValidRemoteToken(token) }

func validSCIMToken(token string) bool { return sharedvalidation.ValidSCIMToken(token) }
