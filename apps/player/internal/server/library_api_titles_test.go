package server_test

import "testing"

func TestKnownTitleSearchRanksLikelyMatchesAcrossAPIAndWeb(t *testing.T) {
	libraryAPIFixture.KnownTitleSearchRanksLikelyMatchesAcrossAPIAndWeb(t)
}

func TestExplicitSortTitleControlsBrowseWithoutChangingDisplayTitle(t *testing.T) {
	libraryAPIFixture.ExplicitSortTitleControlsBrowseWithoutChangingDisplayTitle(t)
}

func TestReleaseFilenameUsesHumanTitleInAPIAndWeb(t *testing.T) {
	libraryAPIFixture.ReleaseFilenameUsesHumanTitleInAPIAndWeb(t)
}

func TestEpisodeFilenamesKeepEpisodeTitlesInAPI(t *testing.T) {
	libraryAPIFixture.EpisodeFilenamesKeepEpisodeTitlesInAPI(t)
}

func TestShortSearchMatchesTitlesAndMetadataWordStarts(t *testing.T) {
	libraryAPIFixture.ShortSearchMatchesTitlesAndMetadataWordStarts(t)
}
