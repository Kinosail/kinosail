package server_test

import (
	"testing"

	"github.com/MikeO7/kinosail/packages/servertest"
)

func TestOIDCCallbackRejectsInvalidQueriesBeforeExchange(t *testing.T) {
	servertest.AssertOIDCCallbackRejectsInvalidQueriesBeforeExchange(t, oidcFixture())
}

func TestOIDCCallbackAcceptsBoundedProviderExtensions(t *testing.T) {
	servertest.AssertOIDCCallbackAcceptsBoundedProviderExtensions(t, oidcFixture())
}

func TestOIDCCallbackValidatesAuthorizationResponseIssuer(t *testing.T) {
	servertest.AssertOIDCCallbackValidatesAuthorizationResponseIssuer(t, oidcFixture())
}
