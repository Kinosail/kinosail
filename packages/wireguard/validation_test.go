package wireguard_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail/packages/wireguard"
)

func TestWireGuardRejectsInvalidConfigurationAndStoredState(t *testing.T) { //nolint:cyclop,gocognit // One table verifies invalid configuration and stored-state boundaries.
	for name, endpoint := range map[string]string{
		"empty": "", "missing port": "host", "missing host": ":51820", "wrong port": "host:1234",
		"space": "bad host:51820", "userinfo": "user@host:51820",
		"control character": "host\ninjected:51820", "oversized host": strings.Repeat("a", 254) + ":51820",
	} {
		t.Run(name, func(t *testing.T) {
			directory := t.TempDir()
			manager, openErr := wireguard.Open(directory, endpoint)
			entries, readErr := os.ReadDir(directory)
			if readErr != nil {
				t.Fatal(readErr)
			}
			if openErr == nil || manager != nil || len(entries) != 0 {
				t.Fatalf("invalid endpoint %q: manager=%t error=%v entries=%d", endpoint, manager != nil, openErr, len(entries))
			}
			for _, path := range []string{"state.json", "wg_confs/kinosail.conf"} {
				if _, statErr := os.Stat(filepath.Join(directory, path)); !os.IsNotExist(statErr) {
					t.Fatalf("rejected endpoint created %s: %v", path, statErr)
				}
			}
		})
	}
	if _, err := wireguard.Open("", "host:51820"); err == nil {
		t.Fatal("empty WireGuard directory was accepted")
	}
	if _, err := wireguard.Open("relative", "host:51820"); err == nil {
		t.Fatal("relative WireGuard directory was accepted")
	}
	for name, state := range map[string]string{"malformed": "{", "empty": `{}`, "unknown": `{"privateKey":"x","publicKey":"x","peers":[],"extra":true}`} {
		t.Run(name, func(t *testing.T) {
			directory := t.TempDir()
			if err := os.WriteFile(filepath.Join(directory, "state.json"), []byte(state), 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := wireguard.Open(directory, "host:51820"); err == nil {
				t.Fatal("invalid state was accepted")
			}
		})
	}
	directory := t.TempDir()
	if err := os.WriteFile(filepath.Join(directory, "state.json"), []byte(strings.Repeat("x", (256<<10)+1)), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := wireguard.Open(directory, "host:51820"); err == nil {
		t.Fatal("oversized WireGuard state was accepted")
	}
}

func TestWireGuardRejectsUnsafeStateFiles(t *testing.T) {
	directory := t.TempDir()
	if _, err := wireguard.Open(directory, "host:51820"); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(directory, "state.json")
	if err := os.Chmod(path, 0o644); err != nil { //nolint:gosec // The test needs deliberately public key material.
		t.Fatal(err)
	}
	if _, err := wireguard.Open(directory, "host:51820"); err == nil {
		t.Fatal("public WireGuard state was accepted")
	}
	if err := os.Chmod(path, 0o600); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(directory, "state-target.json")
	if err := os.Rename(path, target); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, path); err != nil {
		t.Fatal(err)
	}
	if _, err := wireguard.Open(directory, "host:51820"); err == nil {
		t.Fatal("symlinked WireGuard state was accepted")
	}
}

func TestWireGuardRejectsTamperedKeysAndPeers(t *testing.T) {
	directory := t.TempDir()
	manager, err := wireguard.Open(directory, "host:51820")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := manager.Pair("Viewer", "viewer"); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(directory, "state.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var state map[string]any
	if json.Unmarshal(data, &state) != nil {
		t.Fatal("could not decode fixture")
	}
	peers := state["peers"].([]any)
	for name, mutate := range map[string]func(){
		"server public key": func() { state["publicKey"] = strings.Repeat("A", 44) },
		"peer public key":   func() { peers[0].(map[string]any)["publicKey"] = "invalid" },
		"peer address":      func() { peers[0].(map[string]any)["address"] = "10.91.1.2" },
		"peer label":        func() { peers[0].(map[string]any)["label"] = "line\nbreak" },
		"peer profile":      func() { peers[0].(map[string]any)["profileId"] = "" },
	} {
		t.Run(name, func(t *testing.T) {
			var copy map[string]any
			if json.Unmarshal(data, &copy) != nil {
				t.Fatal("could not copy fixture")
			}
			state, peers = copy, copy["peers"].([]any)
			mutate()
			changed, _ := json.Marshal(state)
			if err := os.WriteFile(path, changed, 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := wireguard.Open(directory, "host:51820"); err == nil {
				t.Fatal("tampered state was accepted")
			}
		})
	}
}

func TestWireGuardPairingValidatesLabelsAndRollsBackFailedPersistence(t *testing.T) { //nolint:cyclop,gocognit // One boundary test covers every invalid label and the persistence rollback.
	directory := filepath.Join(t.TempDir(), "wireguard")
	manager, err := wireguard.Open(directory, "host:51820")
	if err != nil {
		t.Fatal(err)
	}
	statePath := filepath.Join(directory, "state.json")
	before, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatal(err)
	}
	for _, label := range []string{"", "   ", "line\nbreak", strings.Repeat("x", 81), string([]byte{0xff})} {
		if _, err := manager.Pair(label, "viewer"); err == nil {
			t.Fatalf("invalid label %q was accepted", label)
		}
	}
	for _, profileID := range []string{"", "bad profile", "line\nbreak", strings.Repeat("x", 129), string([]byte{0xff})} {
		if _, err := manager.Pair("Viewer", profileID); err == nil {
			t.Fatalf("invalid Profile ID %q was accepted", profileID)
		}
	}
	for _, publicKey := range []string{"missing", strings.Repeat("A", 1024)} {
		if err := manager.Revoke(publicKey); err == nil {
			t.Fatalf("invalid Viewer key %q was revoked", publicKey)
		}
	}
	after, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(after, before) || len(manager.Peers()) != 0 {
		t.Fatal("rejected pairing input changed durable or in-memory authorization state")
	}
	if err := os.RemoveAll(directory); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(directory, []byte("blocked"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := manager.Pair("Viewer", "viewer"); err == nil {
		t.Fatal("pairing succeeded without durable state")
	}
	if len(manager.Peers()) != 0 {
		t.Fatal("failed pairing remained authorized")
	}
}

func TestWireGuardRevokeRollsBackFailedPersistence(t *testing.T) {
	directory := filepath.Join(t.TempDir(), "wireguard")
	manager, err := wireguard.Open(directory, "host:51820")
	if err != nil {
		t.Fatal(err)
	}
	pairing, err := manager.Pair("Viewer", "viewer")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(directory); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(directory, []byte("blocked"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := manager.Revoke(pairing.ViewerPublicKey); err == nil {
		t.Fatal("revoke succeeded without durable state")
	}
	if peers := manager.Peers(); len(peers) != 1 || peers[0].PublicKey != pairing.ViewerPublicKey {
		t.Fatalf("failed revoke changed peers: %#v", peers)
	}
}

func TestWireGuardCanRevokeAPeerAfterOtherViewers(t *testing.T) {
	manager, err := wireguard.Open(t.TempDir(), "host:51820")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := manager.Pair("First", "viewer-one"); err != nil {
		t.Fatal(err)
	}
	second, err := manager.Pair("Second", "viewer-two")
	if err != nil {
		t.Fatal(err)
	}
	if err := manager.Revoke(second.ViewerPublicKey); err != nil {
		t.Fatal(err)
	}
}

func TestWireGuardBoundsViewerAddressPool(t *testing.T) {
	manager, err := wireguard.Open(t.TempDir(), "host:51820")
	if err != nil {
		t.Fatal(err)
	}
	for index := range 253 {
		if _, err := manager.Pair("Viewer "+strings.Repeat("x", index%10), "viewer"); err != nil {
			t.Fatalf("pair %d: %v", index, err)
		}
	}
	if _, err := manager.Pair("One too many", "viewer"); err == nil {
		t.Fatal("Viewer address pool exceeded 253 peers")
	}
}
