package productapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/MikeO7/kinosail/packages/identitycore"
)

func TestMeProjectsViewerCapabilities(t *testing.T) { //nolint:cyclop // The projection matrix keeps every response capability explicit and remains below the repository limit.
	t.Parallel()
	tests := map[string]struct {
		viewer                              identitycore.Profile
		owner, downloads, transcode, remote bool
	}{
		"owner":      {identitycore.Profile{ID: "owner", Name: "Owner", Owner: true, Libraries: []string{"Movies"}}, true, true, true, true},
		"scoped key": {identitycore.Profile{ID: "key", Name: "Key", APIKey: true, Scopes: []string{"download", "library"}, Downloads: true, Transcode: true, Remote: true, Rating: "family", AccessStart: "08:00", AccessEnd: "20:00"}, false, true, false, true},
	}
	for name, test := range tests {
		viewer := test.viewer
		t.Run(name, func(t *testing.T) {
			called := false
			handler := Me(func(request *http.Request) MeState {
				called = request.URL.Path == "/api/v1/me"
				return MeState{Server: "Cinema", Viewer: viewer, SSO: true, Language: "es", LanguagePreference: "auto", Languages: []string{"en", "es"}}
			})
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/me", nil))
			var result meResponse
			if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
				t.Fatal(err)
			}
			if response.Code != http.StatusOK || !called || result.Server != "Cinema" || result.Viewer.ID != viewer.ID || result.Viewer.Name != viewer.Name || result.Viewer.Rating != viewer.Rating || result.Viewer.AccessStart != viewer.AccessStart || result.Viewer.AccessEnd != viewer.AccessEnd || result.Viewer.Owner != test.owner || result.Viewer.Downloads != test.downloads || result.Viewer.Transcode != test.transcode || result.Viewer.Remote != test.remote || !result.SSO || result.Language != "es" || result.LanguagePreference != "auto" {
				t.Fatalf("response = %d %#v, called = %v", response.Code, result, called)
			}
		})
	}
}
