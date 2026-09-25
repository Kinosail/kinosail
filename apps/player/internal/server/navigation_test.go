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

func TestDesktopSidebarGroupsConfiguredDestinationsAndKeepsTopSupport(t *testing.T) {
	t.Parallel()
	handler, token := libraryAPIFixture.Server(t)
	response := apiCall(t, handler, token, http.MethodPut, "/api/v1/settings/navigation", map[string]any{"items": []string{"history", "books", "list", "movies"}})
	if response.Code != http.StatusOK {
		t.Fatalf("save navigation = %d %q", response.Code, response.Body.String())
	}
	markup := apiCall(t, handler, token, http.MethodGet, "/?view=movies", nil).Body.String()
	start, end := strings.Index(markup, `<aside class="desktop-sidebar">`), strings.Index(markup, `</aside>`)
	if start < 0 || end < start {
		t.Fatalf("desktop sidebar missing: %q", markup)
	}
	sidebar := markup[start:end]
	for _, expected := range []string{`id="sidebar-library">Library`, `href="/?view=books"`, `href="/?view=movies"`, `id="sidebar-personal">Your library`, `href="/?view=list"`, `id="sidebar-viewing">Library views`, `href="/?view=history"`} {
		position := strings.Index(sidebar, expected)
		if position < 0 {
			t.Fatalf("desktop sidebar lacks %q: %q", expected, sidebar)
		}
		sidebar = sidebar[position+len(expected):]
	}
	for _, check := range []struct {
		body, fragment string
		want           bool
	}{
		{markup[start:end], `class="active" aria-current="page" href="/?view=movies"`, true},
		{markup[start:end], `href="/?view=shows"`, false},
		{markup[end:], `class="header-supporter" href="/supporter">Support Kinosail`, true},
	} {
		if got := strings.Contains(check.body, check.fragment); got != check.want {
			t.Fatalf("desktop navigation fragment %q presence = %t, want %t", check.fragment, got, check.want)
		}
	}
	french := apiCall(t, handler, token, http.MethodGet, "/?lang=fr", nil).Body.String()
	if !strings.Contains(french, `id="sidebar-personal">Votre bibliothèque`) || !strings.Contains(french, `id="sidebar-viewing">Vues de la bibliothèque`) {
		t.Fatalf("sidebar group labels are not localized: %q", french)
	}
}

var mainNavigationMarkup = servertest.MainNavigationMarkup
