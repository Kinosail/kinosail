package dashboard

import (
	"context"
	"errors"
	"net"
	"sync/atomic"
	"testing"
	"time"
)

func TestProbeDialFallsBackFromBlackHoledAddressAndClosesLoser(t *testing.T) {
	winner, peer := net.Pipe()
	defer peer.Close()
	loser, loserPeer := net.Pipe()
	defer loserPeer.Close()
	addresses := []net.IPAddr{{IP: net.ParseIP("fd00::1")}, {IP: net.ParseIP("10.0.0.1")}}
	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	var calls atomic.Int32
	connection, err := dialProbeAddresses(ctx, "tcp", "443", addresses, func(ctx context.Context, _, address string) (net.Conn, error) {
		calls.Add(1)
		if address == "[fd00::1]:443" {
			<-ctx.Done()
			return loser, nil
		}
		if address != "10.0.0.1:443" {
			return nil, errors.New("address was not pinned")
		}
		return winner, nil
	})
	if err != nil || connection != winner || calls.Load() != 2 {
		t.Fatalf("dial = %v, %v; calls %d", connection, err, calls.Load())
	}
	_ = connection.Close()
	_ = loserPeer.SetReadDeadline(time.Now().Add(time.Second))
	if _, err := loserPeer.Read(make([]byte, 1)); err == nil {
		t.Fatal("losing connection remained open")
	} else if timeout, ok := err.(net.Error); ok && timeout.Timeout() {
		t.Fatal("losing connection leaked")
	}
}

func TestProbeDialRejectsUnboundedAddressesBeforeDial(t *testing.T) {
	for _, addresses := range [][]net.IPAddr{nil, make([]net.IPAddr, 65)} {
		calls := 0
		_, err := dialProbeAddresses(t.Context(), "tcp", "443", addresses, func(context.Context, string, string) (net.Conn, error) { calls++; return nil, nil })
		if err == nil || calls != 0 {
			t.Fatal("invalid addresses reached dial")
		}
	}
	for _, addresses := range [][]net.IPAddr{
		{{IP: net.ParseIP("10.0.0.1")}, {IP: net.ParseIP("169.254.169.254")}},
		{{IP: net.ParseIP("fd00::1"), Zone: "en0"}},
	} {
		if err := validateResolvedProbeAddresses("server.local", addresses, nil); err == nil {
			t.Fatal("unsafe address set accepted")
		}
	}
}

func TestProbeDialExhaustsFailuresAndClosesFailedConnections(t *testing.T) {
	for _, dialError := range []error{nil, errors.New("refused")} {
		first, peer := net.Pipe()
		calls := 0
		addresses := []net.IPAddr{{IP: net.ParseIP("10.0.0.1")}, {IP: net.ParseIP("10.0.0.2")}}
		connection, err := dialProbeAddresses(t.Context(), "tcp", "443", addresses, func(context.Context, string, string) (net.Conn, error) {
			calls++
			if calls == 1 {
				return first, errors.New("partial connection")
			}
			return nil, dialError
		})
		if connection != nil || err == nil || calls != 2 {
			t.Fatalf("dial = %v, %v; calls %d", connection, err, calls)
		}
		if dialError != nil && !errors.Is(err, dialError) {
			t.Fatalf("last error lost: %v", err)
		}
		if dialError == nil && err.Error() != "probe could not connect" {
			t.Fatalf("empty result: %v", err)
		}
		assertProbeConnectionClosed(t, peer)
		_ = peer.Close()
	}
}

func TestProbeResolutionRejectsEmptyAndExcessiveAddresses(t *testing.T) {
	for _, addresses := range [][]net.IPAddr{nil, make([]net.IPAddr, 65)} {
		if err := validateResolvedProbeAddresses("server.local", addresses, nil); err == nil {
			t.Fatal("invalid address count accepted")
		}
	}
}

func assertProbeConnectionClosed(t *testing.T, peer net.Conn) {
	t.Helper()
	_ = peer.SetReadDeadline(time.Now().Add(time.Second))
	_, err := peer.Read(make([]byte, 1))
	if err == nil {
		t.Fatal("failed connection remained open")
	}
	var timeout net.Error
	if errors.As(err, &timeout) && timeout.Timeout() {
		t.Fatal("failed connection leaked")
	}
}
