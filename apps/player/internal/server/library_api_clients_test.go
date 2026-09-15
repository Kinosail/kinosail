package server_test

import "testing"

func TestClientCanAuthenticateAndBrowseLibraryWithoutFilesystemPaths(t *testing.T) {
	libraryAPIFixture.ClientCanAuthenticateAndBrowseLibraryWithoutFilesystemPaths(t)
}

func TestClientLibraryRequiresAuthentication(t *testing.T) {
	libraryAPIFixture.ClientLibraryRequiresAuthentication(t)
}

func TestAPIRouterFailuresUseTheJSONContract(t *testing.T) {
	libraryAPIFixture.APIRouterFailuresUseTheJSONContract(t)
}
