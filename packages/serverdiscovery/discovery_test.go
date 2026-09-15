package serverdiscovery

import (
	"context"
	"errors"
	"net"
	"reflect"
	"strings"
	"testing"
	"time"
)

type fakeAdvertisement struct{ stopped chan struct{} }

func (f *fakeAdvertisement) Shutdown() { close(f.stopped) }

func TestAdvertisementLifecycle(t *testing.T) { //nolint:cyclop // Registration, invalid requests, and cleanup share one recorded advertisement.
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	service := &fakeAdvertisement{make(chan struct{})}
	called := false
	err := start(ctx, "Living room", "https://player.example.com:8443/", func(name, kind, domain string, port int, txt []string, interfaces []net.Interface) (advertisement, error) {
		called = true
		if name != "Living room" || kind != ServiceType || domain != "local." || port != 8443 || interfaces != nil || !reflect.DeepEqual(txt, []string{"version=1", "scheme=https", "url=https://player.example.com:8443"}) {
			t.Fatalf("unexpected advertisement: %s %s %s %d %v", name, kind, domain, port, txt)
		}
		return service, nil
	})
	if err != nil || !called {
		t.Fatalf("start: %v, called=%v", err, called)
	}
	select {
	case <-service.stopped:
		t.Fatal("stopped before cancellation")
	default:
	}
	cancel()
	select {
	case <-service.stopped:
	case <-time.After(time.Second):
		t.Fatal("advertisement was not withdrawn")
	}
}

func TestInvalidAdvertisementHasNoNetworkSideEffects(t *testing.T) {
	for _, tc := range []struct{ name, origin string }{
		{"", "http://localhost:38127"},
		{" name", "http://localhost"},
		{strings.Repeat("a", 64), "http://localhost"},
		{"bad\nname", "http://localhost"},
		{string([]byte{0xff}), "http://localhost"},
		{"Player", ""},
		{"Player", "ftp://host.local"},
		{"Player", "https://"},
		{"Player", "https://user:password@host.local"},
		{"Player", "https://host.local/path"},
		{"Player", "https://host.local?"},
		{"Player", "https://host.local#"},
		{"Player", "https://host.local:0"},
		{"Player", "https://host.local:65536"},
		{"Player", "https://host.local:no"},
		{"Player", "https://host.local:"},
		{"Player", "http://public.example.com"},
		{"Player", "http://0.0.0.0:38127"},
		{"Player", "http://[::]:38127"},
		{"Player", " https://host.local"},
		{"Player", "https://" + strings.Repeat("a", 240)},
	} {
		t.Run(tc.name+tc.origin, func(t *testing.T) {
			err := start(t.Context(), tc.name, tc.origin, func(string, string, string, int, []string, []net.Interface) (advertisement, error) {
				t.Fatal("invalid input opened network sockets")
				return nil, nil
			})
			if err == nil {
				t.Fatal("invalid input accepted")
			}
		})
	}
}

func TestLoopbackUsesBonjourHostAndPreservesTLS(t *testing.T) { //nolint:cyclop // Each origin case verifies both advertised hostname and transport.
	for _, origin := range []string{"http://localhost:38127", "http://127.0.0.1:38127", "https://[::1]:38127"} {
		port, txt, err := record("Player", origin)
		if err != nil || port != 38127 || len(txt) != 2 {
			t.Fatalf("%s: %d %v %v", origin, port, txt, err)
		}
		if strings.HasPrefix(origin, "https:") && txt[1] != "scheme=https" {
			t.Fatal("TLS was downgraded")
		}
	}
	for _, origin := range []string{"http://192.168.1.8:38127", "http://server.local:38127", "http://[fd00::1]:38127"} {
		_, txt, err := record("Player", origin)
		if err != nil || len(txt) != 3 || txt[2] != "url="+origin {
			t.Fatalf("%s: %v %v", origin, txt, err)
		}
	}
}

func TestMissingOrCanceledLifecycleAndRegistrationFailure(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	for _, lifecycle := range []context.Context{nil, context.Background(), ctx} {
		if start(lifecycle, "Player", "http://localhost", func(string, string, string, int, []string, []net.Interface) (advertisement, error) {
			t.Fatal("unexpected registration")
			return nil, nil
		}) == nil {
			t.Fatal("invalid lifecycle accepted")
		}
	}
	failure := errors.New("multicast unavailable")
	if err := start(t.Context(), "Player", "http://localhost", func(string, string, string, int, []string, []net.Interface) (advertisement, error) {
		return nil, failure
	}); !errors.Is(err, failure) {
		t.Fatalf("lost error: %v", err)
	}
}

func TestLoopbackAliasIsNarrowAndValidated(t *testing.T) {
	for _, tc := range []struct{ origin, hostname, expected string }{
		{"http://localhost:38127", "Living-Room", "living-room.local"},
		{"https://127.0.0.1:38127", "server", "server.local"},
		{"http://[::1]:38127", "server", "server.local"},
		{"https://player.example.com", "server", ""},
		{"http://192.168.1.8:38127", "server", ""},
		{"http://localhost/path", "server", ""},
		{"http://localhost", "", ""},
		{"http://localhost", "bad name", ""},
		{"http://localhost", "-bad", ""},
		{"http://localhost", strings.Repeat("a", 64), ""},
		{"http://localhost", "bad/name", ""},
	} {
		if actual := loopbackAlias(tc.origin, tc.hostname); actual != tc.expected {
			t.Errorf("%+v: %q", tc, actual)
		}
	}
}
