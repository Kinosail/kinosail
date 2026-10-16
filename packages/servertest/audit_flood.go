package servertest

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func (fixture AuditFixture) RepeatedDenialsFromOneSourceStayBounded(t *testing.T) {
	t.Helper()
	t.Parallel()
	handler := fixture.New("", t.TempDir())
	owner := fixture.SignIn(t, handler, "/setup", "name=Owner&password=owner-password")
	for _, remote := range append(repeatedRemote("203.0.113.9:4321", 100), "198.51.100.7:4321") {
		request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/activity", nil)
		request.RemoteAddr = remote
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != http.StatusUnauthorized {
			t.Fatalf("unauthenticated request = %d %q", response.Code, response.Body.String())
		}
	}
	activity := fixture.CookieRequest(t, handler, http.MethodGet, "/api/v1/activity?category=security&limit=1000", "", owner)
	var view struct {
		Events []struct {
			Action string `json:"action"`
			Remote string `json:"remote"`
		} `json:"events"`
	}
	if err := json.Unmarshal(activity.Body.Bytes(), &view); err != nil {
		t.Fatalf("activity = %d %q: %v", activity.Code, activity.Body.String(), err)
	}
	denied := map[string]int{}
	for _, event := range view.Events {
		if event.Action == "access.denied" {
			denied[event.Remote]++
		}
	}
	if denied["203.0.113.9"] != 20 || denied["198.51.100.7"] != 1 {
		t.Fatalf("denied events by source = %v", denied)
	}
}

func repeatedRemote(remote string, count int) []string {
	remotes := make([]string, count)
	for index := range remotes {
		remotes[index] = remote
	}
	return remotes
}
