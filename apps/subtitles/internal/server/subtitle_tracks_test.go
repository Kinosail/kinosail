package server_test

import "testing"

func TestViewerCanChooseMultipleSubtitleTracks(t *testing.T) {
	t.Parallel()

	libraryAPIFixture.ViewerCanChooseMultipleSubtitleTracks(t)
}
