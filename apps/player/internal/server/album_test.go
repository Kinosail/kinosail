package server_test

import "testing"

func TestViewerCanBrowseAlbumTracksInOrder(t *testing.T) {
	t.Parallel()

	libraryAPIFixture.ViewerCanBrowseAlbumTracksInOrder(t)
}
