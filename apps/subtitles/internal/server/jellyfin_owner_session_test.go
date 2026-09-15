package server_test

import "testing"

func TestJellyfinQuickConnectProjectsOwnerAsMediaViewer(t *testing.T) {
	jellyfinOwnerSessions.JellyfinQuickConnectProjectsOwnerAsMediaViewer(t)
}

func TestJellyfinPasswordSessionProjectsOwnerAsMediaViewer(t *testing.T) {
	jellyfinOwnerSessions.JellyfinPasswordSessionProjectsOwnerAsMediaViewer(t)
}

func TestJellyfinCompatibilitySessionAppearsInActivityWithoutCredentials(t *testing.T) {
	jellyfinOwnerSessions.JellyfinCompatibilitySessionAppearsInActivityWithoutCredentials(t)
}
