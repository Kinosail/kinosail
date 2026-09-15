package wireguard_test

import (
	"strings"
	"testing"

	"github.com/MikeO7/kinosail/packages/wireguard"
)

func TestOwnerCreatesOneTimeViewerPairingLocally(t *testing.T) { //nolint:cyclop // The score of 17 remains below the repository ceiling of 22 for the pairing lifecycle.
	t.Parallel()

	directory := t.TempDir()
	manager, err := wireguard.Open(directory, "media.example.com:51820")
	if err != nil {
		t.Fatal(err)
	}
	first, err := manager.Pair("Family iPhone", "viewer-family")
	if err != nil {
		t.Fatal(err)
	}
	firstPSK := assertInitialPairing(t, first)

	reopened, err := wireguard.Open(directory, "media.example.com:51820")
	if err != nil {
		t.Fatal(err)
	}
	second, err := reopened.Pair("Tablet", "viewer-family")
	if err != nil {
		t.Fatal(err)
	}
	if second.ServerPublicKey != first.ServerPublicKey || !strings.Contains(second.ViewerConfig, "Address = 10.91.0.3/32") || !strings.Contains(second.ServerConfig, first.ViewerPublicKey) {
		t.Fatalf("second pairing = %#v", second)
	}
	if secondPSK := valueFor(second.ViewerConfig, "PresharedKey"); secondPSK == "" || secondPSK == firstPSK {
		t.Fatal("WireGuard peers shared a pre-shared key")
	}
	if err := reopened.Revoke(first.ViewerPublicKey); err != nil {
		t.Fatal(err)
	}
	if peers := reopened.Peers(); len(peers) != 1 || peers[0].Label != "Tablet" || peers[0].ProfileID != "viewer-family" {
		t.Fatalf("peers after revoke = %#v", peers)
	}
	replacement, err := reopened.Pair("Replacement", "viewer-family")
	if err != nil || !strings.Contains(replacement.ViewerConfig, "Address = 10.91.0.2/32") || strings.Contains(replacement.ServerConfig, first.ViewerPublicKey) {
		t.Fatalf("replacement pairing = %#v, %v", replacement, err)
	}
}

func assertInitialPairing(t *testing.T, pairing wireguard.Pairing) string {
	t.Helper()
	if !strings.Contains(pairing.ViewerConfig, "Endpoint = media.example.com:51820") ||
		!strings.Contains(pairing.ViewerConfig, "Address = 10.91.0.2/32") ||
		!strings.Contains(pairing.ViewerConfig, "AllowedIPs = 10.91.0.1/32") ||
		!strings.Contains(pairing.ServerConfig, "AllowedIPs = 10.91.0.2/32") {
		t.Fatalf("pairing = %#v", pairing)
	}
	presharedKey := valueFor(pairing.ViewerConfig, "PresharedKey")
	if presharedKey == "" || presharedKey != valueFor(pairing.ServerConfig, "PresharedKey") {
		t.Fatal("pairing did not contain one matching per-peer pre-shared key")
	}
	if strings.Contains(pairing.ViewerConfig, privateKey(pairing.ServerConfig)) || strings.Contains(pairing.ServerConfig, privateKey(pairing.ViewerConfig)) {
		t.Fatal("pairing crossed private keys")
	}
	return presharedKey
}

func TestOptionalManagerSetup(t *testing.T) {
	t.Parallel()
	if wireguard.OpenOptional("", "media.example.com:51820") != nil || wireguard.OpenOptional("relative", "media.example.com:51820") != nil {
		t.Fatal("unconfigured or invalid optional manager was returned")
	}
	if wireguard.OpenOptional(t.TempDir(), "media.example.com:51820") == nil {
		t.Fatal("configured optional manager was unavailable")
	}
}

func privateKey(config string) string {
	for line := range strings.SplitSeq(config, "\n") {
		if key, ok := strings.CutPrefix(line, "PrivateKey = "); ok {
			return key
		}
	}
	return "missing"
}

func valueFor(config, name string) string {
	for line := range strings.SplitSeq(config, "\n") {
		if value, ok := strings.CutPrefix(line, name+" = "); ok {
			return value
		}
	}
	return ""
}
