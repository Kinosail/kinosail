package servertest

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// AuthCookieRequest preserves each app's existing authenticated form request helper.
type AuthCookieRequest func(*testing.T, http.Handler, string, string, string, *http.Cookie) *httptest.ResponseRecorder

// AssertOwnerCanManageOwnerProfilesThroughWeb preserves the original real-handler authentication regression.
func AssertOwnerCanManageOwnerProfilesThroughWeb(t *testing.T, fixture LibraryAPIFixture, web AuthCookieRequest) {
	t.Parallel()

	dataDir := t.TempDir()
	handler := fixture.NewHandler("", dataDir, true)
	owner := fixture.SignIn(t, handler, "/setup", "name=Owner&password=owner-password")
	settings := web(t, handler, http.MethodGet, "/settings", "", owner)
	if !strings.Contains(settings.Body.String(), `<legend>Profile type</legend>`) || !strings.Contains(settings.Body.String(), `name="owner"`) || !strings.Contains(settings.Body.String(), `value="true"`) || !strings.Contains(settings.Body.String(), `<span>Owner</span>`) || !strings.Contains(settings.Body.String(), `value="false"`) || !strings.Contains(settings.Body.String(), `<span>Viewer</span>`) {
		t.Fatalf("profile type controls = %q", settings.Body.String())
	}
	web(t, handler, http.MethodPost, "/settings/profiles", "name=Partner&password=partner-password&owner=true", owner)
	partner := fixture.SignIn(t, handler, "/login", "name=Partner&password=partner-password")
	if response := web(t, handler, http.MethodGet, "/settings", "", partner); response.Code != http.StatusOK {
		t.Fatalf("second Owner settings = %d %q", response.Code, response.Body.String())
	}
	web(t, handler, http.MethodPost, "/settings/profiles/permissions", "id="+fixture.StoredProfileID(t, dataDir, "Partner")+"&owner=false", owner)
	if response := web(t, handler, http.MethodGet, "/settings", "", partner); response.Code != http.StatusForbidden {
		t.Fatalf("demoted Viewer settings = %d %q", response.Code, response.Body.String())
	}
}

// AssertAdditionalOwnerPersistsAcrossRestart preserves the original real-handler authentication regression.
func AssertAdditionalOwnerPersistsAcrossRestart(t *testing.T, fixture LibraryAPIFixture, web AuthCookieRequest) {
	t.Parallel()

	dataDir := t.TempDir()
	handler := fixture.NewHandler("", dataDir, true)
	owner := fixture.SignIn(t, handler, "/setup", "name=Owner&password=owner-password")
	web(t, handler, http.MethodPost, "/settings/profiles", "name=Partner&password=partner-password&owner=true", owner)

	handler = fixture.NewHandler("", dataDir, true)
	partner := fixture.SignIn(t, handler, "/login", "name=Partner&password=partner-password")
	if response := web(t, handler, http.MethodGet, "/settings", "", partner); response.Code != http.StatusOK {
		t.Fatalf("persisted Owner settings = %d %q", response.Code, response.Body.String())
	}
}
