package server_test

import "testing"

func TestServerAutomaticSkipMapsPlaybackProgressBackToSourceTime(t *testing.T) {
	automaticSkipFixture.ServerAutomaticSkipMapsPlaybackProgressBackToSourceTime(t)
}

func TestVersionedPlaybackMapsProgressBackToSourceTime(t *testing.T) {
	automaticSkipFixture.VersionedPlaybackMapsProgressBackToSourceTime(t)
}

func TestServerAutomaticSkipKeepsSubtitleCuesOnTheShortenedTimeline(t *testing.T) {
	automaticSkipFixture.ServerAutomaticSkipKeepsSubtitleCuesOnTheShortenedTimeline(t)
}
