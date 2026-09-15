package servertest

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

// RouteAuthorizationFixture binds real app construction and the existing stream cancellation writer.
type RouteAuthorizationFixture struct {
	NewHandler func(*testing.T, string) http.Handler
	LoginIP    *atomic.Uint32
	AfterFlush func(*httptest.ResponseRecorder, context.CancelFunc) http.ResponseWriter
}

// Server creates the same authenticated Owner with a confirmed factor and Jellyfin enabled.
func (fixture RouteAuthorizationFixture) Server(t *testing.T) (http.Handler, string) {
	t.Helper()
	handler := fixture.NewHandler(t, t.TempDir())
	secret, token := SetupRouteAuthorization(t, handler, ResponseJSONToken)
	return &RouteAuthorizationServer{Handler: handler, OwnerSecret: secret}, token
}

// Cookie performs the canonical route-fixture form login.
func (fixture RouteAuthorizationFixture) Cookie(t *testing.T, handler http.Handler, name, password string) *http.Cookie {
	t.Helper()
	return LoginRouteCookie(t, handler, name, password, LoginRouteProfile, func() string { return fmt.Sprintf("192.0.2.%d:1234", fixture.LoginIP.Add(1)) })
}

// ExerciseCookie applies the canonical request and original stream-cancel writer.
func (fixture RouteAuthorizationFixture) ExerciseCookie(t *testing.T, handler http.Handler, pattern string, cookie *http.Cookie) *httptest.ResponseRecorder {
	t.Helper()
	return ExerciseRouteWithCookie(t, handler, pattern, cookie, ConcreteRoutePath, fixture.AfterFlush)
}

// CreateRouteProfile creates the original Viewer fixture and checks its status.
func CreateRouteProfile(t *testing.T, handler http.Handler, owner string, profile map[string]any) {
	t.Helper()
	response := RouteJSON(t, handler, owner, http.MethodPost, "/api/v1/profiles", profile)
	if response.Code != http.StatusCreated {
		t.Fatalf("create Viewer Profile = %d %q", response.Code, response.Body.String())
	}
}
