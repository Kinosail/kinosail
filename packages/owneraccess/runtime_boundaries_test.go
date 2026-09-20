package owneraccess

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"testing"
	"time"
)

func TestEndpointMaintenanceUpdatesOnlyCurrentRuntime(t *testing.T) {
	for _, tc := range []struct {
		name             string
		failed, replaced bool
	}{{"healthy", false, false}, {"failure", true, false}, {"replaced", true, true}} {
		t.Run(tc.name, func(t *testing.T) {
			assertEndpointMaintenance(t, tc.failed, tc.replaced)
		})
	}
}

func TestPeerAuthorizationRejectsUnknownAndMalformedAddresses(t *testing.T) {
	config, _, _ := ownerFixture(t)
	peer := Device{ProfileID: "owner", Revision: 1, Address: "10.92.0.2"}
	manager := &Manager{config: config, runtime: &runtime{}, state: state{Enabled: true, Devices: []Device{peer}}}
	for _, address := range []string{"bad", "10.92.0.3:10", "10.92.0.2"} {
		if _, ok := manager.authorizedPeer(address); ok {
			t.Fatalf("authorized %q", address)
		}
	}
	if got, ok := manager.authorizedPeer("10.92.0.2:10"); !ok || got != peer {
		t.Fatal("valid paired device rejected")
	}
	manager.state.Enabled = false
	if _, ok := manager.authorizedPeer("10.92.0.2:10"); ok {
		t.Fatal("disabled management accepted peer")
	}
}

func TestRuntimeAddressAndConnectionCapacity(t *testing.T) {
	current := &runtime{used: map[string]bool{}, connections: map[net.Conn]string{}}
	for range 253 {
		if current.nextAddress() == "" {
			t.Fatal("address pool exhausted early")
		}
	}
	if current.nextAddress() != "" {
		t.Fatal("address pool exceeded subnet")
	}
	connections := make([]net.Conn, 0, 128)
	t.Cleanup(func() {
		for _, connection := range connections {
			_ = connection.Close()
		}
	})
	for range 64 {
		connection, peer := net.Pipe()
		connections = append(connections, connection, peer)
		current.track(connection, http.StateNew)
	}
	rejected, peer := net.Pipe()
	defer peer.Close()
	current.track(rejected, http.StateNew)
	_ = peer.SetReadDeadline(time.Now().Add(time.Second))
	if _, err := peer.Read(make([]byte, 1)); !errors.Is(err, io.EOF) {
		t.Fatal("excess connection retained")
	}
	current.track(connections[0], http.StateHijacked)
	if len(current.connections) != 63 {
		t.Fatal("hijacked connection retained")
	}
}

func assertEndpointMaintenance(t *testing.T, failed, replaced bool) {
	t.Helper()
	config, _, _ := ownerFixture(t)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	calls := 0
	config.MaintainEndpoint = func(_ context.Context, endpoint string) error {
		calls++
		if endpoint != "home.example:51821" {
			t.Errorf("wrong endpoint %q", endpoint)
		}
		cancel()
		if failed {
			return errors.New("maintenance unavailable")
		}
		return nil
	}
	manager := &Manager{config: config, runtime: &runtime{}}
	current := manager.runtime
	if replaced {
		manager.runtime = &runtime{}
	}
	manager.maintainEndpoint(ctx, current, "home.example:51821")
	if calls != 1 || manager.endpointError != (failed && !replaced) {
		t.Fatalf("maintenance calls=%d error=%v", calls, manager.endpointError)
	}
}
