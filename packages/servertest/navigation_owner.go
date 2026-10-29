package servertest

import (
	"net/http"
	"strings"
	"testing"
)

func AssertOwnerCanReachNavigationEditorFromMainBar(t *testing.T, fixture NavigationFixture) {
	handler := fixture.Library.NewHandler("", t.TempDir(), true)
	owner := fixture.Library.SignIn(t, handler, "/setup", "name=Owner&password=owner-password")
	ownerHome := fixture.Web(t, handler, http.MethodGet, "/", "", owner).Body.String()
	for _, expected := range []string{`class="nav-main-supporter" href="/supporter">Supporter`, `class="nav-supporter" href="/supporter">Supporter`, `class="nav-main-edit" href="/settings#navigation"`, `class="nav-edit-menu" href="/settings#navigation"`} {
		if !strings.Contains(ownerHome, expected) {
			t.Fatalf("Owner main navigation lacks %q: %q", expected, MainNavigationMarkup(ownerHome))
		}
	}
	if fixture.CheckSupporterTarget {
		response := fixture.Web(t, handler, http.MethodGet, "/supporter", "", owner)
		if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "Supporter") {
			t.Fatalf("Owner supporter tab target = %d %q", response.Code, response.Body.String())
		}
	}
	assertOwnerNavigationActions(t, ownerHome, fixture.CheckSupporterOrder)
	viewer := fixture.AddViewer(t, handler, owner, "Viewer", "viewer-password")
	viewerHome := fixture.Web(t, handler, http.MethodGet, "/", "", viewer).Body.String()
	viewerMarkup := MainNavigationMarkup(viewerHome)
	if strings.Contains(viewerMarkup, `href="/settings#navigation"`) || strings.Contains(viewerMarkup, `href="/supporter"`) {
		t.Fatalf("Viewer can edit main navigation: %q", MainNavigationMarkup(viewerHome))
	}
}
