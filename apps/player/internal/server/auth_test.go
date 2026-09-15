package server_test

import (
	"testing"

	"github.com/MikeO7/kinosail/packages/servertest"
)

func TestNewInstallationCreatesOwnerProfile(t *testing.T) {
	servertest.AssertNewInstallationCreatesOwnerProfile(t, libraryAPIFixture)
}

func TestViewerCanSignOut(t *testing.T) {
	servertest.AssertViewerCanSignOut(t, libraryAPIFixture)
}

func TestRepeatedLoginAttemptsAreLimited(t *testing.T) {
	servertest.AssertRepeatedLoginAttemptsAreLimited(t, libraryAPIFixture)
}
