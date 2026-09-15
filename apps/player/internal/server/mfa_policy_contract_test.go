package server_test

import (
	"testing"

	"github.com/MikeO7/kinosail/packages/servertest"
)

func TestOwnerCanRequireMFAForEveryProfile(t *testing.T) {
	servertest.AssertOwnerCanRequireMFAForEveryProfile(t, servertest.MFAPolicyFixture{Library: libraryAPIFixture, Web: requestWithCookie})
}

func TestOwnerCanRequireMFAThroughWeb(t *testing.T) {
	servertest.AssertOwnerCanRequireMFAThroughWeb(t, servertest.MFAPolicyFixture{Library: libraryAPIFixture, Web: requestWithCookie})
}
