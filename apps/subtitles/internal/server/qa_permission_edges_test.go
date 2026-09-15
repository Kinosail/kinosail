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

func TestAPIKeyPermissionsHonorScopeAndRouteClassification(t *testing.T) {
	servertest.AssertAPIKeyPermissionsHonorScopeAndRouteClassification(t, func(apiKey bool, scopes []string, scope string, allowed bool) bool {
		return (viewerProfile{APIKey: apiKey, Scopes: scopes}).Permits(scope, allowed)
	}, routeSet)
}
