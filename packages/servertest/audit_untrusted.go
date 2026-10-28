package servertest

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"unicode/utf8"
)

func (fixture AuditFixture) UntrustedRequestTextStaysInActivityJournal(t *testing.T) {
	t.Helper()
	t.Parallel()
	handler := fixture.New("", t.TempDir())
	owner := fixture.SignIn(t, handler, "/setup", "name=Owner&password=owner-password")
	longPath := "/api/v1/devices/" + strings.Repeat("%3C", 5000)
	for _, attempt := range []struct{ method, path, body string }{
		{http.MethodDelete, longPath, ""},
		{http.MethodDelete, "/api/v1/devices/before%00after", ""},
		{http.MethodPost, "/login", "name=" + strings.Repeat("%3C", 5000) + "&password=wrong-password"},
	} {
		request := httptest.NewRequestWithContext(t.Context(), attempt.method, attempt.path, strings.NewReader(attempt.body))
		request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		request.RemoteAddr = "203.0.113.44:4321"
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code < http.StatusBadRequest {
			t.Fatalf("%s %.40s = %d, want rejection", attempt.method, attempt.path, response.Code)
		}
	}
	diagnostics := fixture.CookieRequest(t, handler, http.MethodGet, "/api/v1/diagnostics", "", owner)
	if !strings.Contains(diagnostics.Body.String(), `"activityHealthy":true`) || !strings.Contains(diagnostics.Body.String(), `"activityFailures":0`) {
		t.Fatalf("diagnostics after untrusted request text = %d %.600s", diagnostics.Code, diagnostics.Body.String())
	}
	activity := fixture.CookieRequest(t, handler, http.MethodGet, "/api/v1/activity?category=security&limit=1000", "", owner)
	var view struct {
		Events []struct{ Action, Target, Remote, Actor string } `json:"events"`
	}
	if err := json.Unmarshal(activity.Body.Bytes(), &view); err != nil {
		t.Fatalf("activity = %d: %v", activity.Code, err)
	}
	var long, nul bool
	for _, event := range view.Events {
		if event.Remote != "203.0.113.44" {
			continue
		}
		for _, value := range []string{event.Target, event.Actor} {
			if len(value) > 4096 || strings.ContainsRune(value, 0) || !utf8.ValidString(value) {
				t.Fatalf("activity kept unbounded request text: %d bytes", len(value))
			}
		}
		long = long || strings.HasPrefix(event.Target, "/api/v1/devices/<<<")
		nul = nul || strings.HasPrefix(event.Target, "/api/v1/devices/before") && strings.HasSuffix(event.Target, "after")
	}
	if !long || !nul {
		t.Fatalf("security activity missing bounded denials: long=%t nul=%t events=%d", long, nul, len(view.Events))
	}
}
