package passkeys

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestRegistrationRequiresPriorAuthenticationAtBothSteps(t *testing.T) {
	fixture, flow := newFlowFixture(t)
	flow.config.RecentlyAuthenticated = func(*http.Request, time.Duration) bool { return false }
	for _, handler := range []http.HandlerFunc{flow.BeginRegistration, flow.FinishRegistration} {
		response := httptest.NewRecorder()
		handler(response, httptest.NewRequestWithContext(t.Context(), http.MethodPost, "http://localhost:38127/api/v1/passkeys/register/begin", nil))
		if response.Code != http.StatusForbidden || fixture.adds != 0 || fixture.strong != 0 || len(fixture.engine.ceremonies) != 0 {
			t.Fatalf("stale session changed enrollment: %d %#v", response.Code, fixture)
		}
	}
}
