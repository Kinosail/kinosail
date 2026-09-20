package publicgateway

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
)

func TestHostnameBoundary(t *testing.T) {
	for _, host := range []string{"", "localhost", "127.0.0.1", "EXAMPLE.com", "a..com", "-a.com", "a-.com", "a_.com", strings.Repeat("a", 64) + ".com", strings.Repeat("a.", 127)} {
		if validHostname(host) {
			t.Errorf("accepted %q", host)
		}
		if err := Run(t.Context(), host); err == nil {
			t.Errorf("Run accepted %q", host)
		}
	}
	for _, host := range []string{"a.example", "a-1.example", strings.Repeat("a", 63) + ".example"} {
		if !validHostname(host) {
			t.Errorf("rejected %q", host)
		}
	}
}

func TestGatewayUnavailableDoesNotExposeSocket(t *testing.T) {
	r := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "https://family.example:443/", nil)
	r.URL.Scheme = ""
	r.URL.Host = ""
	w := httptest.NewRecorder()
	Handler("family.example", "/absent-private-socket").ServeHTTP(w, r)
	if w.Code != http.StatusServiceUnavailable || strings.Contains(w.Body.String(), "absent-private") {
		t.Fatalf("response %d %s", w.Code, w.Body.String())
	}
}

type failingReader struct{}

func (failingReader) Read([]byte) (int, error) { return 0, errors.New("private detail") }
func TestBoundedResponse(t *testing.T) {
	for _, tc := range []struct {
		reader io.Reader
		want   string
		fail   bool
	}{
		{strings.NewReader("abc"), "abc", false}, {strings.NewReader("abcd"), "", true}, {failingReader{}, "", true},
	} {
		data, err := readBounded(tc.reader, 3)
		if (err != nil) != tc.fail || string(data) != tc.want {
			t.Fatalf("data=%q error=%v", data, err)
		}
	}
}

type limitConn struct {
	net.Conn
	address net.Addr
	closed  bool
}

func (c *limitConn) RemoteAddr() net.Addr { return c.address }
func (c *limitConn) Close() error         { c.closed = true; return nil }
func TestConnectionLimitsReleaseCapacity(t *testing.T) {
	var limits connectionLimits
	conns := make([]*limitConn, 0, 256)
	for i := range 256 {
		c := &limitConn{address: &net.TCPAddr{IP: net.ParseIP("192.0.2." + strconv.Itoa(i/32+1)), Port: 1000 + i}}
		limits.track(c, http.StateNew)
		limits.track(c, http.StateActive)
		if c.closed {
			t.Fatalf("closed allowed connection %d", i)
		}
		conns = append(conns, c)
	}
	extra := &limitConn{address: &net.TCPAddr{IP: net.ParseIP("192.0.2.99"), Port: 1}}
	limits.track(extra, http.StateNew)
	if !extra.closed {
		t.Fatal("global limit not enforced")
	}
	for i, c := range conns {
		state := http.StateClosed
		if i%2 == 0 {
			state = http.StateHijacked
		}
		limits.track(c, state)
	}
	if len(limits.connections) != 0 || len(limits.sources) != 0 {
		t.Fatal("closed connections retained capacity")
	}
	replacement := &limitConn{address: conns[0].address}
	limits.track(replacement, http.StateNew)
	if replacement.closed {
		t.Fatal("released capacity unavailable")
	}
}

func TestRunReportsListenerConflict(t *testing.T) {
	listener, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp", "127.0.0.1:8443")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	if err = Run(context.Background(), "family.example"); err == nil {
		t.Fatal("listener conflict ignored")
	}
}

func TestConnectionLimitsPerSource(t *testing.T) {
	var limits connectionLimits
	address := &net.TCPAddr{IP: net.ParseIP("192.0.2.1"), Port: 1234}
	for range 32 {
		limits.track(&limitConn{address: address}, http.StateNew)
	}
	extra := &limitConn{address: address}
	limits.track(extra, http.StateNew)
	if !extra.closed {
		t.Fatal("per-source limit not enforced")
	}
	limits.track(extra, http.StateClosed)
	if len(limits.connections) != 32 {
		t.Fatal("rejected connection changed accounting")
	}
}
