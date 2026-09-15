package identitycore

import (
	"context"
	"errors"
	"net"
	"net/http"
	"testing"
	"time"
)

func TestAllowedResolvedAddress(t *testing.T) {
	t.Parallel()
	if _, err := AllowedResolvedAddress(nil, func(net.IP) bool { return true }); err == nil {
		t.Fatal("empty DNS result was allowed")
	}
	if _, err := AllowedResolvedAddress([]net.IP{net.ParseIP("203.0.113.1")}, nil); err == nil {
		t.Fatal("missing policy was allowed")
	}
	addresses := []net.IP{net.ParseIP("203.0.113.1"), net.ParseIP("127.0.0.1")}
	if _, err := AllowedResolvedAddress(addresses, AllowedOutboundIP); err == nil {
		t.Fatal("partially prohibited DNS result was allowed")
	}
	address, err := AllowedResolvedAddress(addresses[:1], AllowedOutboundIP)
	if err != nil || !address.Equal(addresses[0]) {
		t.Fatalf("public DNS result = %v, %v", address, err)
	}
}

func TestAddressPolicies(t *testing.T) {
	t.Parallel()
	tests := []struct {
		address     string
		integration bool
		outbound    bool
	}{
		{"203.0.113.1", true, true},
		{"192.168.1.1", true, false},
		{"127.0.0.1", true, false},
		{"100.64.0.1", true, false},
		{"100.63.255.255", true, true},
		{"100.127.255.255", true, false},
		{"100.128.0.1", true, true},
		{"0.0.0.0", false, false},
		{"224.0.0.1", false, false},
		{"169.254.1.1", false, false},
		{"ff02::1", false, false},
		{"fe80::1", false, false},
		{"::", false, false},
		{"", false, false},
	}
	for _, test := range tests {
		address := net.ParseIP(test.address)
		if AllowedIntegrationIP(address) != test.integration || AllowedOutboundIP(address) != test.outbound {
			t.Fatalf("policies for %q = integration %v, outbound %v", test.address, AllowedIntegrationIP(address), AllowedOutboundIP(address))
		}
	}
	if cgnatIP(nil) || cgnatIP(net.ParseIP("::1")) {
		t.Fatal("non-IPv4 address was classified as carrier-grade NAT")
	}
}

func TestOutboundHTTPClientConfiguration(t *testing.T) {
	t.Parallel()
	client := OutboundHTTPClient(3*time.Second, AllowedOutboundIP)
	if client.Timeout != 3*time.Second || client.Transport == nil {
		t.Fatalf("client configuration = %#v", client)
	}
	request, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "https://example.com", nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := client.CheckRedirect(request, nil); !errors.Is(err, http.ErrUseLastResponse) {
		t.Fatalf("redirect policy = %v", err)
	}
}

func TestOutboundHTTPClientDialPolicy(t *testing.T) {
	t.Parallel()
	transport := OutboundHTTPClient(time.Second, func(net.IP) bool { return true }).Transport.(*http.Transport)
	if _, err := transport.DialContext(t.Context(), "tcp", "missing-port"); err == nil {
		t.Fatal("invalid address was dialed")
	}
	if _, err := transport.DialContext(t.Context(), "tcp", "invalid\x00host:80"); err == nil {
		t.Fatal("invalid host was resolved")
	}
	denied := OutboundHTTPClient(time.Second, func(net.IP) bool { return false }).Transport.(*http.Transport)
	if _, err := denied.DialContext(t.Context(), "tcp", "127.0.0.1:80"); err == nil {
		t.Fatal("denied address was dialed")
	}
	listener, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	accepted := make(chan net.Conn, 1)
	go func() {
		connection, acceptErr := listener.Accept()
		if acceptErr == nil {
			accepted <- connection
		}
	}()
	connection, err := transport.DialContext(context.Background(), "tcp", listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	_ = connection.Close()
	select {
	case server := <-accepted:
		_ = server.Close()
	case <-time.After(time.Second):
		t.Fatal("local test connection was not accepted")
	}
}
