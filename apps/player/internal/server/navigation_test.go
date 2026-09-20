package server_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail-player/internal/server"
	"github.com/MikeO7/kinosail/packages/servertest"
)

func navigationFixture() servertest.NavigationFixture {
	return servertest.NavigationFixture{LibraryLink: `>Browse library</a>`, Library: libraryAPIFixture, Web: requestWithCookie, AddViewer: addAndSignInViewer, CheckSupporterTarget: false, CheckSupporterOrder: true, RequiredCSS: "/static/app.css?v=", ForbiddenCSS: ""}
}

func TestOwnerCanCustomizeLibraryNavigationThroughAPIAndWeb(t *testing.T) {
	servertest.AssertOwnerCanCustomizeLibraryNavigationThroughAPIAndWeb(t, navigationFixture())
}

func TestNavigationValidationRejectsInvalidValuesWithoutSideEffects(t *testing.T) {
	servertest.AssertNavigationValidationRejectsInvalidValuesWithoutSideEffects(t, navigationFixture())
}

func TestCustomizedNavigationPersistsAcrossRestart(t *testing.T) {
	servertest.AssertCustomizedNavigationPersistsAcrossRestart(t, navigationFixture())
}

func TestQuickConnectUsesJellyfinProductNameInNavigation(t *testing.T) {
	t.Parallel()
	handler := server.New(server.Config{DataDir: t.TempDir()})
	markup := mainNavigationMarkup(apiCall(t, handler, "", http.MethodGet, "/", nil).Body.String())
	if !strings.Contains(markup, `href="/quick-connect">Quick Connect`) || strings.Contains(markup, `href="/quick-connect">Connect`) {
		t.Fatalf("Quick Connect navigation label = %q", markup)
	}
}

func TestLibrarySearchUsesLocalizedAccessibleCopy(t *testing.T) {
	t.Parallel()
	handler := server.New(server.Config{DataDir: t.TempDir()})
	page := apiCall(t, handler, "", http.MethodGet, "/?lang=fr", nil)
	assertAPIBody(t, page, http.StatusOK,
		`<label for="library-search">Rechercher dans la bibliothèque</label>`,
		`aria-label="Rechercher dans toutes les bibliothèques"`,
		`placeholder="Rechercher dans toutes les bibliothèques"`)
	if strings.Contains(page.Body.String(), "Find something") {
		t.Fatalf("search control retains untranslated copy: %q", page.Body.String())
	}
}

var mainNavigationMarkup = servertest.MainNavigationMarkup
