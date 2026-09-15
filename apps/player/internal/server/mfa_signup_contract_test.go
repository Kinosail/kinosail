package server_test

import (
	"testing"

	"github.com/MikeO7/kinosail/packages/servertest"
)

func TestSingleOwnerMustEnrollOneAuthenticationFactor(t *testing.T) {
	servertest.AssertSingleOwnerMustEnrollOneAuthenticationFactor(t, servertest.MFAPolicyFixture{Library: libraryAPIFixture, Web: requestWithCookie})
}

func TestOwnerCanChooseTOTPWhenSigningUpThroughWebAndAPI(t *testing.T) {
	servertest.AssertOwnerCanChooseTOTPWhenSigningUpThroughWebAndAPI(t, servertest.MFAPolicyFixture{Library: libraryAPIFixture, Web: requestWithCookie})
}
