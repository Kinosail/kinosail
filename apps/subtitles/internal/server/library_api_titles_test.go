package server_test

import "testing"

func TestExplicitSortTitleControlsBrowseWithoutChangingDisplayTitle(t *testing.T) {
	libraryAPIFixture.ExplicitSortTitleControlsBrowseWithoutChangingDisplayTitle(t)
}

func TestEpisodeFilenamesKeepEpisodeTitlesInAPI(t *testing.T) {
	libraryAPIFixture.EpisodeFilenamesKeepEpisodeTitlesInAPI(t)
}
