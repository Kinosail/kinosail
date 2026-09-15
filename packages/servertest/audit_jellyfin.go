package servertest

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func JellyfinSessionAuditClassifiesAndRedactsCredentials(t *testing.T, auditAction func(*http.Request) (string, string), auditDetails func(*http.Request) map[string]string) {
	t.Helper()
	t.Parallel()

	for name, test := range map[string]struct {
		path, body, channel string
	}{
		"password":      {"/Users/AuthenticateByName", `{"Username":"Owner","Pw":"secret-password"}`, "jellyfin-compatibility"},
		"quick connect": {"/Users/AuthenticateWithQuickConnect", `{"Secret":"secret-token"}`, "jellyfin-quick-connect"},
	} {
		t.Run(name, func(t *testing.T) {
			request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, test.path, strings.NewReader(test.body))
			request.Header.Set("Content-Type", "application/json")
			action, category := auditAction(request)
			details := auditDetails(request)
			if action != "session.created" || category != "security" || details["channel"] != test.channel || details["privileges"] != "media-only" {
				t.Fatalf("audit = %q %q %#v", action, category, details)
			}
			if strings.Contains(strings.Join(auditDetailValues(details), " "), "secret") {
				t.Fatalf("audit details contain a credential: %#v", details)
			}
		})
	}
}

func auditDetailValues(values map[string]string) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		result = append(result, value)
	}
	return result
}
