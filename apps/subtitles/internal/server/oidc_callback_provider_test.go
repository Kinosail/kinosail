package server_test

import (
	"testing"

	"github.com/MikeO7/kinosail/packages/servertest"
)

func TestOIDCRequestsOnlyTheIdentityScopeItUses(t *testing.T) {
	servertest.AssertOIDCRequestsOnlyTheIdentityScopeItUses(t, oidcFixture())
}

func TestOIDCRejectsMissingConfiguredIdentityClaimWithoutLinking(t *testing.T) {
	servertest.AssertOIDCRejectsMissingConfiguredIdentityClaimWithoutLinking(t, oidcFixture())
}

func TestOIDCSupportsClientSecretPostProviders(t *testing.T) {
	servertest.AssertOIDCSupportsClientSecretPostProviders(t, oidcFixture())
}

func TestOIDCCallbackHandlesProviderErrorWithoutExchange(t *testing.T) {
	servertest.AssertOIDCCallbackHandlesProviderErrorWithoutExchange(t, oidcFixture())
}
