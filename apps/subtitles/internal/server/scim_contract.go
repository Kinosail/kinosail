package server

import "github.com/MikeO7/kinosail/packages/scim"

type SCIMConfig = scim.Config

type (
	scimProfileInput = scim.ProfileInput
	scimProfileName  = scim.Name
	scimProfileEmail = scim.Email
)

var errSCIMConflict = scim.ErrConflict
