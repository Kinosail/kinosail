package owneraccess

import (
	"context"
	"net"
	"strings"
	"time"

	"github.com/MikeO7/kinosail/packages/httpguard"
	"golang.org/x/net/dns/dnsmessage"
)

// The tunnel resolves only A/AAAA records. The Server name stays inside the tunnel;
// ordinary device DNS lookups use the host resolver, with bounded work and replies.
func (m *Manager) serveDNS(ctx context.Context, runtime *runtime, hostname string) {
	var limiter httpguard.Limiter
	capacity := make(chan struct{}, 8)
	for {
		buf := make([]byte, 1232)
		n, address, err := runtime.dns.ReadFrom(buf)
		if err != nil {
			return
		}
		peer, ok := m.authorizedPeer(address.String())
		if !ok || !limiter.Allow(peer.PublicKey, 600) {
			continue
		}
		select {
		case capacity <- struct{}{}:
		default:
			continue
		}
		go func() {
			defer func() { <-capacity }()
			lookupCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
			defer cancel()
			reply := dnsReply(lookupCtx, buf[:n], hostname, net.DefaultResolver.LookupIP)
			if len(reply) > 0 {
				_, _ = runtime.dns.WriteTo(reply, address)
			}
		}()
	}
}

func dnsReply(ctx context.Context, packet []byte, hostname string, lookup func(context.Context, string, string) ([]net.IP, error)) []byte {
	var request dnsmessage.Message
	if len(packet) > 1232 || request.Unpack(packet) != nil || request.Header.Response || request.Header.OpCode != 0 || len(request.Questions) != 1 || len(request.Answers) != 0 || len(request.Authorities) != 0 {
		return nil
	}
	q := request.Questions[0]
	name := strings.TrimSuffix(strings.ToLower(q.Name.String()), ".")
	response := dnsmessage.Message{Header: dnsmessage.Header{ID: request.ID, Response: true, RecursionDesired: request.RecursionDesired, RecursionAvailable: true}, Questions: request.Questions}
	if !validHostname(name) || q.Class != dnsmessage.ClassINET || q.Type != dnsmessage.TypeA && q.Type != dnsmessage.TypeAAAA {
		response.RCode = dnsmessage.RCodeRefused
	} else {
		var ips []net.IP
		var err error
		if name == hostname {
			if q.Type == dnsmessage.TypeA {
				ips = []net.IP{net.ParseIP(Address)}
			}
		} else {
			network := "ip4"
			if q.Type == dnsmessage.TypeAAAA {
				network = "ip6"
			}
			ips, err = lookup(ctx, network, name)
		}
		if err != nil {
			response.RCode = dnsmessage.RCodeServerFailure
		}
		for i, ip := range ips {
			if i >= 8 {
				break
			}
			resource := dnsmessage.Resource{Header: dnsmessage.ResourceHeader{Name: q.Name, Type: q.Type, Class: q.Class, TTL: 5}}
			if q.Type == dnsmessage.TypeA && ip.To4() != nil {
				var a [4]byte
				copy(a[:], ip.To4())
				resource.Body = &dnsmessage.AResource{A: a}
			} else if q.Type == dnsmessage.TypeAAAA && ip.To16() != nil && ip.To4() == nil {
				var a [16]byte
				copy(a[:], ip.To16())
				resource.Body = &dnsmessage.AAAAResource{AAAA: a}
			} else {
				continue
			}
			response.Answers = append(response.Answers, resource)
		}
	}
	reply, err := response.Pack()
	if err != nil || len(reply) > 1232 {
		return nil
	}
	return reply
}
