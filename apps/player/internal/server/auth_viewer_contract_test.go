package server_test

import (
	"testing"

	"github.com/MikeO7/kinosail/packages/servertest"
)

func TestOwnerCanCreateViewerProfileWithoutGrantingAdministration(t *testing.T) {
	servertest.AssertOwnerCanCreateViewerProfileWithoutGrantingAdministration(t, libraryAPIFixture)
}

func TestOwnerCanRemoveViewerAndRevokeSessions(t *testing.T) {
	servertest.AssertOwnerCanRemoveViewerAndRevokeSessions(t, libraryAPIFixture)
}

func TestOwnerCanResetViewerPassword(t *testing.T) {
	servertest.AssertOwnerCanResetViewerPassword(t, libraryAPIFixture)
}

func TestOwnerControlsViewerDownloads(t *testing.T) {
	servertest.AssertOwnerControlsViewerDownloads(t, libraryAPIFixture)
}
