package server

import "github.com/MikeO7/kinosail/packages/scim"

// SCIMConfig configures the optional SCIM 2.0 provisioning endpoint.
type SCIMConfig = scim.Config

type (
	scimProfileInput = scim.ProfileInput
	scimProfileName  = scim.Name
	scimProfileEmail = scim.Email
)

var errSCIMConflict = scim.ErrConflict
