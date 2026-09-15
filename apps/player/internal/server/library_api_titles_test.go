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
