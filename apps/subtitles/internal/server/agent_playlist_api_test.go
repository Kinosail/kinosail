package server_test

import (
	"testing"
)

func TestAgentCanCreateAnOrderedPlaylistInOneAPICall(t *testing.T) {
	libraryAPIFixture.AgentCanCreateOrderedPlaylist(t)
}
