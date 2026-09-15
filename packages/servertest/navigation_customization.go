package servertest

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
)

// AssertOwnerCanCustomizeLibraryNavigationThroughAPIAndWeb retains the original navigation regression.
func AssertOwnerCanCustomizeLibraryNavigationThroughAPIAndWeb(t *testing.T, fixture NavigationFixture) {
	t.Parallel()
	handler, token := fixture.Library.Server(t)

	AssertAPIBody(t, APICall(t, handler, token, http.MethodGet, "/api/v1/settings", nil), http.StatusOK,
		`"navigation":["home","list","movies","shows","music","audiobooks","books","photos","collections","playlists","unwatched","history"]`)
	AssertAPIBody(t, APICall(t, handler, token, http.MethodPut, "/api/v1/settings/navigation", map[string]any{
		"items": []string{"movies", "home", "history", "books", "music", "photos"},
	}), http.StatusOK, `"status":"saved"`)

	home := APICall(t, handler, token, http.MethodGet, "/", nil)
	markup := MainNavigationMarkup(home.Body.String())
	for _, hidden := range []string{"My List", "Shows", "Collections"} {
		if strings.Contains(markup, `>`+hidden+`</a>`) {
			t.Fatalf("hidden navigation item %q remains in %q", hidden, markup)
		}
	}
	for _, ordered := range []string{`href="/?view=movies">Movies`, `href="/?view=all">Home`, `href="/?view=history">History`, `href="/?view=books">Books`, `class="nav-main-overflow" href="/?view=music">Music`, `class="nav-main-overflow" href="/?view=photos">Photos`, `<details class="nav-more">`} {
		position := strings.Index(markup, ordered)
		if position < 0 {
			t.Fatalf("custom navigation lacks %q: %q", ordered, markup)
		}
		markup = markup[position+len(ordered):]
	}

	settings := APICall(t, handler, token, http.MethodGet, "/settings", nil)
	AssertAPIBody(t, settings, http.StatusOK, `id="navigation"`, `action="/settings/navigation"`, `Show Movies`, `Move Movies down`)
	updated := WebFormCall(t, handler, token, "/settings/navigation", url.Values{
		"items": {"movies", "home", "history", "books", "music", "photos"},
		"move":  {"history:up"},
	})
	if updated.Code != http.StatusSeeOther || updated.Header().Get("Location") != "/settings#navigation" {
		t.Fatalf("web navigation update = %d %q %q", updated.Code, updated.Header().Get("Location"), updated.Body.String())
	}
	AssertAPIBody(t, APICall(t, handler, token, http.MethodGet, "/api/v1/settings", nil), http.StatusOK,
		`"navigation":["movies","history","home","books","music","photos"]`)
	AssertAPIBody(t, APICall(t, handler, token, http.MethodGet, "/api/v1/activity", nil), http.StatusOK, `"action":"settings.navigation.updated"`)
}

// AssertNavigationValidationRejectsInvalidValuesWithoutSideEffects retains the original navigation regression.
func AssertNavigationValidationRejectsInvalidValuesWithoutSideEffects(t *testing.T, fixture NavigationFixture) {
	t.Parallel()
	handler, token := fixture.Library.Server(t)
	before := `"navigation":["home","list","movies","shows","music","audiobooks","books","photos","collections","playlists","unwatched","history"]`

	invalid := []any{
		[]string{},
		[]string{"home", "home"},
		[]string{"home", "unknown"},
		[]string{"home", "list", "movies", "shows", "music", "audiobooks", "books", "photos", "collections", "playlists", "unwatched", "history", "extra"},
	}
	for _, items := range invalid {
		response := APICall(t, handler, token, http.MethodPut, "/api/v1/settings/navigation", map[string]any{"items": items})
		if response.Code != http.StatusBadRequest {
			t.Fatalf("invalid navigation %#v = %d %q", items, response.Code, response.Body.String())
		}
		AssertAPIBody(t, APICall(t, handler, token, http.MethodGet, "/api/v1/settings", nil), http.StatusOK, before)
	}

	for name, form := range map[string]url.Values{
		"missing":   {},
		"duplicate": {"items": {"home", "home"}},
		"unknown":   {"items": {"home", "unknown"}},
		"bad move":  {"items": {"home", "movies"}, "move": {"unknown:up"}},
		"extra key": {"items": {"home"}, "unexpected": {"value"}},
	} {
		response := WebFormCall(t, handler, token, "/settings/navigation", form)
		if response.Code != http.StatusBadRequest {
			t.Fatalf("%s navigation form = %d %q", name, response.Code, response.Body.String())
		}
		AssertAPIBody(t, APICall(t, handler, token, http.MethodGet, "/api/v1/settings", nil), http.StatusOK, before)
	}
}

// AssertCustomizedNavigationPersistsAcrossRestart retains the original navigation regression.
func AssertCustomizedNavigationPersistsAcrossRestart(t *testing.T, fixture NavigationFixture) {
	t.Parallel()
	data := t.TempDir()
	handler := fixture.Library.NewHandler("", data, false)
	response := WebFormCall(t, handler, "", "/settings/navigation", url.Values{"items": {"home", "books", "movies"}})
	if response.Code != http.StatusSeeOther {
		t.Fatalf("save navigation = %d %q", response.Code, response.Body.String())
	}

	home := MainNavigationMarkup(APICall(t, fixture.Library.NewHandler("", data, false), "", http.MethodGet, "/", nil).Body.String())
	homePosition, booksPosition, moviesPosition := strings.Index(home, `href="/?view=all">Home`), strings.Index(home, `href="/?view=books">Books`), strings.Index(home, `href="/?view=movies">Movies`)
	if homePosition >= booksPosition || booksPosition >= moviesPosition || strings.Contains(home, `>Shows</a>`) {
		t.Fatalf("persisted navigation = %q", home)
	}
}
