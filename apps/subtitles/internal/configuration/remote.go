package configuration

import (
	"github.com/MikeO7/kinosail/packages/configurationcore"
)

func validateRemoteAccess(configured Snapshot) error {
	return configurationcore.ValidateRemoteAccess(configured.String)
}

func validRemoteToken(token string) bool { return configurationcore.ValidRemoteToken(token) }
