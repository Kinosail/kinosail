package server

import (
	"testing"

	"github.com/MikeO7/kinosail/packages/servertest"
)

func TestRequestLoggingRecoversPanicsAndRedactsURLs(t *testing.T) {
	servertest.RequestLoggingRecoversPanicsAndRedactsURLs(t, observedAuditHandler, setPrivateAuditViewer)
}

func TestUnknownJellyfinRouteLoggingKeepsOnlySafeRouteSegments(t *testing.T) {
	servertest.UnknownJellyfinRouteLoggingKeepsOnlySafeRouteSegments(t, observedAuditHandler)
}
