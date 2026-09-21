package servertest

import (
	"net/http"
	"strings"
	"testing"
)

// NavigationFixture keeps real app construction and explicit app navigation policies.
type NavigationFixture struct {
	Library                                   LibraryAPIFixture
	Web                                       AuthCookieRequest
	AddViewer                                 func(*testing.T, http.Handler, *http.Cookie, string, string) *http.Cookie
	CheckSupporterTarget, CheckSupporterOrder bool
	LibraryLink, RequiredCSS, ForbiddenCSS    string
}

// MainNavigationMarkup isolates the original main-bar fragment.
func MainNavigationMarkup(page string) string {
	start := strings.Index(page, `<nav aria-label="Main navigation"`)
	if start < 0 {
		return ""
	}
	end := strings.Index(page[start:], `</nav>`)
	if end < 0 {
		return ""
	}
	return page[start : start+end]
}

func assertOwnerNavigationActions(t *testing.T, ownerHome string, supporterOrder bool) {
	t.Helper()
	supportPosition := strings.Index(ownerHome, `class="nav-main-supporter" href="/supporter">Supporter`)
	actionsPosition := strings.Index(ownerHome, `class="nav-more-section nav-more-actions"`)
	editPosition := strings.Index(ownerHome, `class="nav-edit-menu" href="/settings#navigation"`)
	accountPosition := strings.Index(ownerHome, `class="nav-more-section nav-more-account"`)
	if (supporterOrder && (supportPosition < 0 || editPosition < supportPosition)) || actionsPosition < 0 || editPosition < actionsPosition || (accountPosition >= 0 && accountPosition > actionsPosition) {
		t.Fatalf("Owner mobile navigation editor is outside Actions: %q", MainNavigationMarkup(ownerHome))
	}
}
