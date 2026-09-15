package server_test

import "testing"

func TestAutomaticPlaybackKeepsDirectTimelineAndOffersCompatibleFallback(t *testing.T) {
	automaticSkipFixture.AutomaticPlaybackKeepsDirectTimelineAndOffersCompatibleFallback(t)
}

func TestServerAutomaticSkipRenditionRemovesConfiguredRanges(t *testing.T) {
	automaticSkipFixture.ServerAutomaticSkipRenditionRemovesConfiguredRanges(t)
}

func TestAutomaticSkipUsesExactTranscodeAwayFromRandomAccessPoints(t *testing.T) {
	automaticSkipFixture.AutomaticSkipUsesExactTranscodeAwayFromRandomAccessPoints(t)
}
