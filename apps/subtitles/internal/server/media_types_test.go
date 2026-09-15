package server_test

import (
	"testing"

	"github.com/MikeO7/kinosail/packages/servertest"
)

func TestViewerCanBrowseAndOpenMusicAndPhotos(t *testing.T) {
	libraryAPIFixture.ViewerCanBrowseAndOpenMusicAndPhotos(t)
}

var mustContainAll = servertest.MustContainAll
