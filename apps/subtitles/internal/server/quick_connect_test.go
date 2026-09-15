package server_test

import (
	"net/http"
	"testing"
	"time"

	"github.com/MikeO7/kinosail-subtitles/internal/server"
	"github.com/MikeO7/kinosail/packages/servertest"
)

func quickConnectFixture() servertest.QuickConnectFixture {
	return servertest.QuickConnectFixture{
		New: func(data string, ttl time.Duration) http.Handler {
			return server.New(server.Config{DataDir: data, RequireAuth: true, QuickConnectTTL: ttl})
		},
		SignIn: signInTestProfile, AddViewer: addTestViewer, Web: requestWithCookie, APIKey: apiKeyRequest, API: apiCall,
	}
}

func TestViewerApprovesOneTimeQuickConnectSession(t *testing.T) {
	servertest.AssertViewerApprovesOneTimeQuickConnectSession(t, quickConnectFixture(), false)
}

func TestApprovedQuickConnectCannotBeReassignedToAnotherViewer(t *testing.T) {
	servertest.AssertApprovedQuickConnectCannotBeReassignedToAnotherViewer(t, quickConnectFixture())
}

func TestQuickConnectCodeExpires(t *testing.T) {
	servertest.AssertQuickConnectCodeExpires(t, quickConnectFixture())
}

func TestQuickConnectCreationIsRateLimited(t *testing.T) {
	servertest.AssertQuickConnectCreationIsRateLimited(t, quickConnectFixture())
}

func TestQuickConnectRejectsInvalidDeviceBeforeCreatingRequest(t *testing.T) {
	servertest.AssertQuickConnectRejectsInvalidDeviceBeforeCreatingRequest(t, quickConnectFixture())
}
