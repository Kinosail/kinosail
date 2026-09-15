package owneraccess

import (
	"context"
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
