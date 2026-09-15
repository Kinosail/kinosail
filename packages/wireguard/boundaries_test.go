package wireguard_test

import (
	"strings"
	"testing"

	"github.com/MikeO7/kinosail/packages/wireguard"
)

func TestWireGuardAcceptsValidEndpointFormsAtTheLengthBoundary(t *testing.T) {
	maximumDNSName := strings.Repeat("a", 63) + "." + strings.Repeat("b", 63) + "." + strings.Repeat("c", 63) + "." + strings.Repeat("d", 61)
	for _, endpoint := range []string{maximumDNSName + ":51820", "192.0.2.1:51820", "[2001:db8::1]:51820"} {
		if manager, err := wireguard.Open(t.TempDir(), endpoint); err != nil || manager == nil {
			t.Fatalf("valid endpoint %q: manager=%#v error=%v", endpoint, manager, err)
		}
	}
}

func TestWireGuardAcceptsViewerFieldsAtTheLengthBoundary(t *testing.T) {
	manager, err := wireguard.Open(t.TempDir(), "host:51820")
	if err != nil {
		t.Fatal(err)
	}
	label, profileID := strings.Repeat("l", 80), strings.Repeat("p", 128)
	if _, err = manager.Pair(label, profileID); err != nil {
		t.Fatal(err)
	}
	peers := manager.Peers()
	if len(peers) != 1 || peers[0].Label != label || peers[0].ProfileID != profileID {
		t.Fatalf("boundary Viewer = %#v", peers)
	}
}
