package server_test

import (
	"testing"

	"github.com/MikeO7/kinosail/packages/servertest"
)

func TestOwnerCanRemoveViewerAndRevokeSessions(t *testing.T) {
	servertest.AssertOwnerCanRemoveViewerAndRevokeSessions(t, libraryAPIFixture)
}

func TestOwnerCanResetViewerPassword(t *testing.T) {
	servertest.AssertOwnerCanResetViewerPassword(t, libraryAPIFixture)
}
