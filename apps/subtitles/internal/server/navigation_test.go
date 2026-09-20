package server_test

import (
	"testing"

	"github.com/MikeO7/kinosail/packages/servertest"
)

func navigationFixture() servertest.NavigationFixture {
	return servertest.NavigationFixture{LibraryLink: `href="/?view=library"`, Library: libraryAPIFixture, Web: requestWithCookie, AddViewer: addAndSignInViewer, CheckSupporterTarget: true, CheckSupporterOrder: false, RequiredCSS: "/static/app.css?v="}
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
