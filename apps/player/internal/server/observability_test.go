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

func TestJellyfinStreamShapeDistinguishesSafeHLSPaths(t *testing.T) {
	servertest.JellyfinStreamShapeDistinguishesSafeHLSPaths(t, jellyfinStreamShape)
}

func TestJellyfinSourceHLSFileRejectsUnsafeChildren(t *testing.T) {
	servertest.JellyfinSourceHLSFileRejectsUnsafeChildren(t, jellyfinSourceHLSFile)
}
