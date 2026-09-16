package server

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/MikeO7/kinosail/packages/library"
	"github.com/MikeO7/kinosail/packages/mediashares"
	sharedpasskeys "github.com/MikeO7/kinosail/packages/passkeys"
	"github.com/MikeO7/kinosail/packages/quickconnect"
	"github.com/MikeO7/kinosail/packages/servertest"
)

func TestMediaShareContract(t *testing.T) {
	servertest.MediaShareContract(t, func(directory string, items []library.Item, ready bool) *mediashares.Store {
		return newMediaShares(directory, nil, memoryLibraryIndex(items, ready))
	})
}

func TestKillSwitchRevokesEveryPublicAuthorizationArtifact(t *testing.T) { //nolint:cyclop // One integration flow proves every public authorization artifact is revoked together.
	directory := t.TempDir()
	profiles := newProfileStore(directory)
	owner, err := newProfile("Owner", "owner-password", true)
	if err != nil || addTestOwner(profiles, owner) != nil {
		t.Fatal(err)
	}
	viewerID, err := profiles.addProfile("Viewer", "viewer-password", false, profilePolicy{Remote: true, Rating: "family", Libraries: []string{"all"}})
	if err != nil {
		t.Fatal(err)
	}
	viewer, _ := profiles.byID(viewerID)
	profiles.profiles[1].TOTPSecret = "secret"
	if _, err = profiles.sessionModule().CreateStrongPublic(viewerID, "Browser", true); err != nil {
		t.Fatal(err)
	}
	quick := newQuickConnect(time.Minute)
	secret, _, err := quick.connections.Create(quickconnect.Request{Device: "Remote", Remote: true})
	if err != nil {
		t.Fatal(err)
	}
	index := memoryLibraryIndex([]library.Item{{ID: "item", Path: filepath.Join(directory, "item.mp4")}}, true)
	shares := newMediaShares(directory, nil, index)
	if _, _, err = shares.Create([]string{"item"}, time.Hour, 1, true); err != nil {
		t.Fatal(err)
	}
	passkeys := newPasskeyAuth("https://kinosail.test", profiles)
	_, publicCookie, err := passkeys.engine.BeginLogin(sharedpasskeys.Ceremony{CookiePath: "/api/v1/passkeys/", Secure: true, Public: true})
	if err != nil {
		t.Fatal(err)
	}
	_, localCookie, err := passkeys.engine.BeginLogin(sharedpasskeys.Ceremony{CookiePath: "/api/v1/passkeys/", Secure: true})
	if err != nil {
		t.Fatal(err)
	}
	if err = revokePublicAuthorization(profiles, passkeys, quick, shares); err != nil {
		t.Fatal(err)
	}
	_, found := quick.connections.Status(secret)
	servertest.AssertPublicAuthorizationRevoked(t, servertest.PublicAuthorizationArtifacts{
		Sessions: len(profiles.sessions), QuickPresent: found, Passkeys: passkeys.engine,
		PublicCookie: publicCookie, LocalCookie: localCookie, Shares: shares,
	})
	updated, _ := profiles.byID(viewerID)
	if updated.Revision <= viewer.Revision {
		t.Fatalf("remote profile revision = %d, want greater than %d", updated.Revision, viewer.Revision)
	}
}
