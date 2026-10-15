package server_test

import (
	"testing"

	"github.com/MikeO7/kinosail/packages/servertest"
)

func TestOwnerCanRequireMFAThroughWeb(t *testing.T) {
	servertest.AssertOwnerCanRequireMFAThroughWeb(t, servertest.MFAPolicyFixture{Library: libraryAPIFixture, Web: requestWithCookie})
}
