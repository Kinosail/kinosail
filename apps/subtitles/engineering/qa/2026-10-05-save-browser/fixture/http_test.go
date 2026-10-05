package main

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"time"
)

// The socket target is allocated before any root input is read. Destination
// URLs come from this finite inventory, never from a fixture or request URL.
type ownedTarget struct {
	tls               *httptest.Server
	listener          *checkedListener
	origin, authority string
	mu                sync.Mutex
	routes            map[string]url.URL
	item, receipt     string
	stopOnce          sync.Once
	closeOK           bool
}

func newOwnedTarget() (*ownedTarget, error) {
	tls := httptest.NewUnstartedServer(nil)
	listener := &checkedListener{Listener: tls.Listener}
	tls.Listener = listener
	authority := listener.Addr().String()
	target := &ownedTarget{
		tls: tls, listener: listener, authority: authority,
		origin: "https://" + authority, routes: make(map[string]url.URL),
		closeOK: true,
	}
	if !ownedLoopbackAuthority(authority) {
		if !target.stop() {
			return nil, errors.New("fixture listener close failed")
		}
		return nil, errors.New("fixture listener is not owned loopback")
	}
	target.seedFixedRoutes()
	return target, nil
}

func ownedLoopbackAuthority(authority string) bool {
	host, port, err := net.SplitHostPort(authority)
	if err != nil || port == "" {
		return false
	}
	address := net.ParseIP(host)
	return address != nil && address.IsLoopback()
}

func (target *ownedTarget) seedFixedRoutes() {
	for _, path := range []string{
		"/setup", "/login", "/account", "/account/mfa/enable", "/?view=library",
		"/api/v1/subtitle-library?view=library", "/api/v1/subtitle-library?view=history",
		"/api/v1/subtitle-operations",
	} {
		target.addRouteLocked(path)
	}
}

func (target *ownedTarget) addRouteLocked(path string) {
	route, query, _ := strings.Cut(path, "?")
	target.routes[path] = url.URL{Scheme: "https", Host: target.authority, Path: route, RawQuery: query}
}

func (target *ownedTarget) endpoint(path string) (string, error) {
	target.mu.Lock()
	endpoint, exists := target.routes[path]
	target.mu.Unlock()
	if !exists {
		return "", errors.New("private route is not admitted")
	}
	return endpoint.String(), nil
}

func (target *ownedTarget) ownsPath(path string) bool {
	target.mu.Lock()
	_, exists := target.routes[path]
	target.mu.Unlock()
	return exists
}

func (target *ownedTarget) ownsItem(id string) bool {
	target.mu.Lock()
	defer target.mu.Unlock()
	return target.item != "" && id == target.item
}

func (target *ownedTarget) privateClient(timeout time.Duration) *http.Client {
	client := *target.tls.Client()
	original, ok := client.Transport.(*http.Transport)
	if !ok {
		client.Transport = deniedTransport{}
		return &client
	}
	transport := original.Clone()
	transport.DialContext = target.dialOwned
	client.Transport = transport
	client.Timeout = timeout
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	return &client
}

func (target *ownedTarget) dialOwned(ctx context.Context, network, address string) (net.Conn, error) {
	if address != target.authority {
		return nil, errors.New("private transport authority rejected")
	}
	dialer := net.Dialer{Timeout: 2 * time.Second}
	return dialer.DialContext(ctx, network, address)
}

func (target *ownedTarget) admittedRequest(request *http.Request) bool {
	if request.URL == nil {
		return false
	}
	endpoint := request.URL
	if endpoint.Scheme != "https" || endpoint.Host != target.authority || endpoint.User != nil {
		return false
	}
	address := net.ParseIP(endpoint.Hostname())
	if address == nil || !address.IsLoopback() {
		return false
	}
	return target.ownsPath(endpoint.RequestURI()) && endpoint.Fragment == "" && endpoint.Opaque == "" && endpoint.RawPath == ""
}

func (f *fixture) admittedPrivateRequest(request *http.Request) bool {
	return f.target != nil && f.target.admittedRequest(request)
}

func (f *fixture) closeFailedPrivateResponse(response *http.Response) {
	if response == nil {
		return
	}
	if err := response.Body.Close(); err != nil {
		f.failBoundary()
	}
}

func readPrivateResponse(response *http.Response) ([]byte, error) {
	data, readErr := io.ReadAll(io.LimitReader(response.Body, privateResponseLimit+1))
	return data, errors.Join(readErr, response.Body.Close())
}

type deniedTransport struct{}

func (deniedTransport) RoundTrip(*http.Request) (*http.Response, error) {
	return nil, errors.New("private TLS transport unavailable")
}
