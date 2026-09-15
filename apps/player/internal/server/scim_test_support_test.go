package server_test

import "github.com/MikeO7/kinosail/packages/servertest"

const (
	scimTestToken  = "scim-test-token-012345678901234567890" //nolint:gosec // Test-only bearer token.
	scimUserSchema = "urn:ietf:params:scim:schemas:core:2.0:User"
)

var scimCall = servertest.SCIMCall
