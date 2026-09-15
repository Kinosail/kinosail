package server_test

import "testing"

func TestLeanBackPlaybackDefaultsAreOnAndCanBeDisabled(t *testing.T) {
	libraryAPIFixture.LeanBackPlaybackDefaults(t)
}
