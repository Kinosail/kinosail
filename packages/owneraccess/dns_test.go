package owneraccess

import (
	"context"
	"errors"
	"net"
	"testing"

	"golang.org/x/net/dns/dnsmessage"
)

func TestTunnelDNSPinsManagementNameWithoutForwardingIt(t *testing.T) {
	for _, kind := range []dnsmessage.Type{dnsmessage.TypeA, dnsmessage.TypeAAAA} {
		name, _ := dnsmessage.NewName("server.example.")
		question := dnsmessage.Message{Header: dnsmessage.Header{ID: 12}, Questions: []dnsmessage.Question{{Name: name, Type: kind, Class: dnsmessage.ClassINET}}}
		raw, err := question.Pack()
		if err != nil {
			t.Fatal(err)
		}
		reply := dnsReply(t.Context(), raw, "server.example", func(context.Context, string, string) ([]net.IP, error) {
			t.Fatal("management lookup escaped tunnel")
			return nil, nil
		})
		var response dnsmessage.Message
		if response.Unpack(reply) != nil || response.ID != 12 || !response.Response {
			t.Fatal("invalid DNS reply")
		}
		if kind == dnsmessage.TypeA {
			if len(response.Answers) != 1 || response.Answers[0].Body.(*dnsmessage.AResource).A != [4]byte{10, 92, 0, 1} {
				t.Fatal("wrong management address")
			}
		} else if len(response.Answers) != 0 {
			t.Fatal("AAAA escaped private IPv4 route")
		}
	}
	called := false
	if reply := dnsReply(t.Context(), make([]byte, 1233), "server.example", func(context.Context, string, string) ([]net.IP, error) { called = true; return nil, nil }); len(reply) != 0 || called {
		t.Fatal("oversized DNS performed work")
	}
}

func TestTunnelDNSBoundsAndFiltersResolverAnswers(t *testing.T) {
	for _, tc := range []struct {
		name, network string
		kind          dnsmessage.Type
		failed        bool
		count         int
		code          dnsmessage.RCode
	}{
		{"ipv4", "ip4", dnsmessage.TypeA, false, 6, dnsmessage.RCodeSuccess},
		{"ipv6", "ip6", dnsmessage.TypeAAAA, false, 1, dnsmessage.RCodeSuccess},
		{"resolver failure", "ip4", dnsmessage.TypeA, true, 0, dnsmessage.RCodeServerFailure},
	} {
		t.Run(tc.name, func(t *testing.T) {
			name, _ := dnsmessage.NewName("external.example.")
			request := dnsmessage.Message{Questions: []dnsmessage.Question{{Name: name, Type: tc.kind, Class: dnsmessage.ClassINET}}}
			raw, err := request.Pack()
			if err != nil {
				t.Fatal(err)
			}
			calls := 0
			reply := dnsReply(t.Context(), raw, "server.example", func(_ context.Context, network, host string) ([]net.IP, error) {
				calls++
				if network+":"+host != tc.network+":external.example" {
					t.Errorf("lookup %s %s", network, host)
				}
				return resolverFixture(tc.failed)
			})
			var response dnsmessage.Message
			if err = response.Unpack(reply); err != nil {
				t.Fatal(err)
			}
			if calls != 1 || response.RCode != tc.code || len(response.Answers) != tc.count {
				t.Fatalf("calls=%d response=%+v", calls, response)
			}
			assertDNSMetadata(t, response.Answers, tc.kind)
		})
	}
}

func resolverFixture(failed bool) ([]net.IP, error) {
	if failed {
		return nil, errors.New("resolver failed")
	}
	ips := []net.IP{nil, net.ParseIP("192.0.2.1"), net.ParseIP("2001:db8::1")}
	for range 10 {
		ips = append(ips, net.ParseIP("192.0.2.2"))
	}
	return ips, nil
}

func assertDNSMetadata(t *testing.T, answers []dnsmessage.Resource, kind dnsmessage.Type) {
	t.Helper()
	for _, answer := range answers {
		if answer.Header.TTL != 5 || answer.Header.Type != kind {
			t.Fatal("wrong answer metadata")
		}
	}
}
