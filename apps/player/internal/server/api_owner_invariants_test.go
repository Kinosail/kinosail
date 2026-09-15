package server_test

import "testing"

func TestServerKeepsAtLeastOneOwner(t *testing.T) { identityAPIFixture.ServerKeepsAtLeastOneOwner(t) }

func TestOwnerCanRemoveAnotherOwnerButNotTheLast(t *testing.T) {
	identityAPIFixture.OwnerCanRemoveAnotherOwnerButNotTheLast(t)
}
