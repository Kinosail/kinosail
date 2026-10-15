package server_test

import (
	"testing"

	"github.com/MikeO7/kinosail/packages/servertest"
)

func navigationFixture() servertest.NavigationFixture {
	return servertest.NavigationFixture{LibraryLink: `href="/?view=library"`, Library: libraryAPIFixture, Web: requestWithCookie, AddViewer: addAndSignInViewer, CheckSupporterTarget: true, CheckSupporterOrder: false, RequiredCSS: "/static/app.css?v=", NoQuickConnectOrShares: true}
}

func TestNavigationValidationRejectsInvalidValuesWithoutSideEffects(t *testing.T) {
	servertest.AssertNavigationValidationRejectsInvalidValuesWithoutSideEffects(t, navigationFixture())
}
