package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/MikeO7/kinosail/packages/servertest"
)

var routeAuthorizationFixture = servertest.RouteAuthorizationFixture{
	NewHandler: func(t *testing.T, data string) http.Handler {
		t.Helper()
		return New(Config{DataDir: data, RequireAuth: true, ProxyToken: "trusted-test-proxy", Configuration: homeAssistantRouteConfiguration(t, data)})
	},
	LoginIP: &routeLoginIP,
	AfterFlush: func(response *httptest.ResponseRecorder, cancel context.CancelFunc) http.ResponseWriter {
		return &cancelAfterFlushWriter{response, cancel}
	},
}
