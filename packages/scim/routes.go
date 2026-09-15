package scim

const (
	serviceProviderPattern = "GET /scim/v2/ServiceProviderConfig"
	resourceTypesPattern   = "GET /scim/v2/ResourceTypes"
	resourceTypePattern    = "GET /scim/v2/ResourceTypes/{id}"
	schemasPattern         = "GET /scim/v2/Schemas"
	schemaPattern          = "GET /scim/v2/Schemas/{id}"
	usersPattern           = "GET /scim/v2/Users"
	createUserPattern      = "POST /scim/v2/Users"
	userPattern            = "GET /scim/v2/Users/{id}"
	replaceUserPattern     = "PUT /scim/v2/Users/{id}"
	patchUserPattern       = "PATCH /scim/v2/Users/{id}"
	deleteUserPattern      = "DELETE /scim/v2/Users/{id}"
)

var routePatterns = []string{serviceProviderPattern, resourceTypesPattern, resourceTypePattern, schemasPattern, schemaPattern, usersPattern, createUserPattern, userPattern, replaceUserPattern, patchUserPattern, deleteUserPattern}

// Patterns returns every route pattern installed by Register.
func Patterns() []string { return append([]string(nil), routePatterns...) }
