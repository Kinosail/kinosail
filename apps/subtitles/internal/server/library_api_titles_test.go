package server_test

import "testing"

func TestExplicitSortTitleControlsBrowseWithoutChangingDisplayTitle(t *testing.T) {
	libraryAPIFixture.ExplicitSortTitleControlsBrowseWithoutChangingDisplayTitle(t)
}

func TestEpisodeFilenamesKeepEpisodeTitlesInAPI(t *testing.T) {
	libraryAPIFixture.EpisodeFilenamesKeepEpisodeTitlesInAPI(t)
}

func TestShortSearchMatchesTitlesAndMetadataWordStarts(t *testing.T) {
	libraryAPIFixture.ShortSearchMatchesTitlesAndMetadataWordStarts(t)
}
