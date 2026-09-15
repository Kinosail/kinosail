package server_test

import (
	"testing"

	"github.com/MikeO7/kinosail/packages/servertest"
)

func TestOwnerCanReachNavigationEditorFromMainBar(t *testing.T) {
	servertest.AssertOwnerCanReachNavigationEditorFromMainBar(t, navigationFixture())
}

func TestMobileMoreMenuPlacesLibraryAfterSecondaryActions(t *testing.T) {
	servertest.AssertMobileMoreMenuPlacesLibraryAfterSecondaryActions(t, navigationFixture())
}

func TestEveryMainNavigationDestinationRetainsTheMainBar(t *testing.T) {
	servertest.AssertEveryMainNavigationDestinationRetainsTheMainBar(t, navigationFixture())
}

func TestSignedInApplicationPagesRetainTheMainBar(t *testing.T) {
	servertest.AssertSignedInApplicationPagesRetainTheMainBar(t, navigationFixture())
}
