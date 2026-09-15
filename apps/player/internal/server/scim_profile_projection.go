package server

import (
	"github.com/MikeO7/kinosail/packages/scim"
	scimapp "github.com/MikeO7/kinosail/packages/scim/app"
)

func scimProtocolProfile(profile viewerProfile) scim.Profile {
	return scimapp.ProjectProfile(profile.ID, profile.Name, profile.SCIMUserName, profile.SCIMExternalID, profile.SCIMName, profile.SCIMEmails, profile.SCIMEnterprise, profile.Disabled, profile.SCIMCreatedAt, profile.SCIMUpdatedAt, profile.Revision)
}
