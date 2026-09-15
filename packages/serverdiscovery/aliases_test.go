package serverdiscovery

import (
	"net"
	"reflect"
	"testing"
)

func TestLocalAliasesUseOnlyOwnPrivateAddressesAndExpectedPort(t *testing.T) {
	addresses := []net.Addr{
		&net.IPNet{IP: net.ParseIP("192.168.1.8")}, &net.IPNet{IP: net.ParseIP("192.168.1.8")},
		&net.IPNet{IP: net.ParseIP("fd00::1")}, &net.IPNet{IP: net.ParseIP("127.0.0.1")},
		&net.IPNet{IP: net.ParseIP("8.8.8.8")}, &net.IPNet{IP: net.ParseIP("0.0.0.0")},
		&net.IPNet{IP: net.ParseIP("fe80::1")}, (*net.IPNet)(nil), nil,
	}
	expected := []string{"server.local:38127", "192.168.1.8:38127", "[fd00::1]:38127"}
	if actual := localAliases("http://localhost:38127", "server.local", addresses); !reflect.DeepEqual(actual, expected) {
		t.Fatalf("aliases = %v", actual)
	}
	for _, origin := range []string{"https://player.example.com", "http://192.168.1.8:38127", "http://localhost/path", "bad"} {
		if aliases := localAliases(origin, "server.local", addresses); len(aliases) != 0 {
			t.Fatalf("unexpected aliases for %s: %v", origin, aliases)
		}
	}
	if aliases := localAliases("http://localhost:38127", "bad/host", addresses); len(aliases) != 0 {
		t.Fatalf("invalid hostname accepted: %v", aliases)
	}
}

func TestLocalAliasesHandleImplicitPortsAndBoundCardinality(t *testing.T) {
	if actual := localAliases("https://localhost", "server.local", nil); !reflect.DeepEqual(actual, []string{"server.local:443", "server.local"}) {
		t.Fatalf("aliases = %v", actual)
	}
	if actual := localAliases("http://localhost", "server.local", nil); !reflect.DeepEqual(actual, []string{"server.local:80", "server.local"}) {
		t.Fatalf("aliases = %v", actual)
	}
	addresses := []net.Addr{}
	for i := 1; i <= 40; i++ {
		addresses = append(addresses, &net.IPNet{IP: net.IPv4(10, 0, 0, byte(i))})
	}
	if aliases := localAliases("http://localhost:38127", "server.local", addresses); len(aliases) != 33 {
		t.Fatalf("alias cap: %d", len(aliases))
	}
	if aliases := localAliases("http://localhost:38127", "server.local", make([]net.Addr, 257)); len(aliases) != 1 {
		t.Fatalf("oversized interface list: %v", aliases)
	}
}
