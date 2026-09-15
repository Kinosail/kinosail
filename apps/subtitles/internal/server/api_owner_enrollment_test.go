package server_test

import "testing"

func TestOwnerCanCreateAnotherOwnerThroughAPI(t *testing.T) {
	identityAPIFixture.OwnerCanCreateAnotherOwnerThroughAPI(t)
}

func TestOwnerCanChangeProfileTypeThroughAPI(t *testing.T) {
	identityAPIFixture.OwnerCanChangeProfileTypeThroughAPI(t)
}
