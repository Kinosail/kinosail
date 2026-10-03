package server

import (
	"testing"

	"github.com/MikeO7/kinosail/packages/servertest"
)

func TestPasskeyBeginRoutesFailClosedWhenConfigurationIsInvalid(t *testing.T) {
	t.Parallel()
	auth := newPasskeyAuth("://invalid", &profileStore{})
	servertest.AssertPasskeyBeginRoutesFailClosedWhenConfigurationIsInvalid(t, auth.beginRegistration, auth.beginLogin)
}
