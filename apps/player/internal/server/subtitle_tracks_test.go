package server_test

import "testing"

func TestViewerCanChooseMultipleSubtitleTracks(t *testing.T) {
	t.Parallel()

	libraryAPIFixture.ViewerCanChooseMultipleSubtitleTracks(t)
}

func TestLegacySubtitleFilesAreServedAsUTF8(t *testing.T) {
	t.Parallel()
	libraryAPIFixture.LegacySubtitleFilesAreServedAsUTF8(t)
}
