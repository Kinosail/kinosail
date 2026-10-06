package main

import (
	"context"
	"crypto/tls"
	"errors"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
)

type r16CheckedListener struct {
	net.Listener
	once sync.Once
	err  error
}

func (listener *r16CheckedListener) Close() error {
	listener.once.Do(func() { listener.err = listener.Listener.Close() })
	return listener.err
}

type r16Target struct {
	server    *httptest.Server
	listener  *r16CheckedListener
	origin    string
	authority string
	lifecycle context.Context
	client    *http.Client
	mu        sync.Mutex
	item      string
	csrf      string
	bad       bool
	joined    bool
	active    int
	changed   chan struct{}
	streams   map[*r16CapturedStream]bool
	stopOnce  sync.Once
	stopped   bool
}

func r16NewTarget(lifecycle context.Context) (*r16Target, error) {
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		return nil, errors.New("owned R16 listener unavailable")
	}
	checked := &r16CheckedListener{Listener: listener}
	hosted := httptest.NewUnstartedServer(nil)
	if err = hosted.Listener.Close(); err != nil {
		return nil, errors.Join(err, checked.Close())
	}
	hosted.Listener = checked
	authority := listener.Addr().String()
	return &r16Target{
		server: hosted, listener: checked,
		origin: "https://" + authority, authority: authority,
		lifecycle: lifecycle, changed: make(chan struct{}, 1),
		streams: make(map[*r16CapturedStream]bool),
	}, nil
}

func (target *r16Target) start(handler http.Handler) {
	target.server.Config.Handler = handler
	target.server.Config.BaseContext = func(net.Listener) context.Context { return target.lifecycle }
	target.server.StartTLS()
	transport := target.server.Client().Transport.(*http.Transport).Clone()
	transport.TLSClientConfig = transport.TLSClientConfig.Clone()
	transport.TLSClientConfig.MinVersion = tls.VersionTLS12
	transport.MaxResponseHeaderBytes = 16384
	transport.Proxy = nil
	transport.DialContext = target.dial
	jar, err := cookiejar.New(nil)
	if err != nil {
		target.fail()
		transport.CloseIdleConnections()
		return
	}
	target.client = &http.Client{
		Transport: transport, Jar: jar,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
}

func (target *r16Target) dial(ctx context.Context, network, authority string) (net.Conn, error) {
	if !strings.HasPrefix(network, "tcp") || authority != target.authority {
		return nil, errors.New("owned R16 dial boundary rejected")
	}
	host, _, err := net.SplitHostPort(authority)
	if err != nil || !net.ParseIP(host).IsLoopback() {
		return nil, errors.New("owned R16 loopback boundary rejected")
	}
	return (&net.Dialer{}).DialContext(ctx, "tcp4", target.authority)
}

func (target *r16Target) admitted(request *http.Request) bool {
	if request.URL == nil || request.URL.Scheme != "https" ||
		request.URL.Host != target.authority || request.URL.User != nil ||
		request.URL.Fragment != "" || request.Host != target.authority {
		return false
	}
	host, _, err := net.SplitHostPort(request.URL.Host)
	return err == nil && net.ParseIP(host).IsLoopback() &&
		target.routeAllowed(request.Method, request.URL.RequestURI())
}

func (target *r16Target) endpoint(method, route string) (string, error) {
	if !target.routeAllowed(method, route) {
		return "", errors.New("owned R16 route rejected")
	}
	value, err := url.ParseRequestURI(route)
	if err != nil || value.IsAbs() || value.Host != "" || value.Fragment != "" {
		return "", errors.New("owned R16 route rejected")
	}
	return target.origin + route, nil
}

func (target *r16Target) fail() {
	target.mu.Lock()
	target.bad = true
	target.mu.Unlock()
}

func (target *r16Target) track(delta int) {
	target.mu.Lock()
	target.active += delta
	target.mu.Unlock()
	select {
	case target.changed <- struct{}{}:
	default:
	}
}
