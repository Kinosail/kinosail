package server_test

import (
	"testing"

	"github.com/MikeO7/kinosail/packages/servertest"
)

func TestOwnerCanManageOwnerProfilesThroughWeb(t *testing.T) {
	servertest.AssertOwnerCanManageOwnerProfilesThroughWeb(t, libraryAPIFixture, requestWithCookie)
}

func TestAdditionalOwnerPersistsAcrossRestart(t *testing.T) {
	servertest.AssertAdditionalOwnerPersistsAcrossRestart(t, libraryAPIFixture, requestWithCookie)
}
