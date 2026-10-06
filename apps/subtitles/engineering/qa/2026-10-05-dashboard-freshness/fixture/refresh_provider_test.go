package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io"
	"net"
	"net/http"
	"sync"
	"time"
)

type r16ProviderPeer struct {
	listener *r16CheckedListener
	server   *http.Server
	origin   string
	key      string
	mu       sync.Mutex
	requests int
	active   int
	bad      bool
	done     chan struct{}
	stopOnce sync.Once
	stopped  bool
	joined   bool
}

func r16NewProviderPeer(lifecycle context.Context) (*r16ProviderPeer, error) {
	data := make([]byte, 24)
	if _, err := rand.Read(data); err != nil {
		return nil, errors.New("owned R16 provider key unavailable")
	}
	listener, err := (&net.ListenConfig{}).Listen(lifecycle, "tcp4", "127.0.0.1:0")
	if err != nil {
		return nil, errors.New("owned R16 provider listener unavailable")
	}
	checked := &r16CheckedListener{Listener: listener}
	peer := &r16ProviderPeer{
		listener: checked, origin: "http://" + listener.Addr().String(),
		key: hex.EncodeToString(data), done: make(chan struct{}),
	}
	peer.server = &http.Server{
		Handler:           http.HandlerFunc(peer.serve),
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       5 * time.Second,
		BaseContext:       func(net.Listener) context.Context { return lifecycle },
	}
	go peer.run()
	return peer, nil
}

func (peer *r16ProviderPeer) serve(writer http.ResponseWriter, request *http.Request) {
	peer.mu.Lock()
	peer.requests++
	peer.active++
	peer.mu.Unlock()
	defer func() {
		peer.mu.Lock()
		peer.active--
		peer.mu.Unlock()
	}()
	if !peer.admitted(request) {
		peer.fail()
		writer.WriteHeader(http.StatusForbidden)
		return
	}
	writer.Header().Set("Content-Type", "application/json")
	writer.Header().Set("X-RateLimit-Remaining", "17")
	writer.WriteHeader(http.StatusOK)
	if _, err := io.WriteString(writer, "{}"); err != nil {
		peer.fail()
	}
}

func (peer *r16ProviderPeer) admitted(request *http.Request) bool {
	if request.Method != http.MethodGet || request.URL.Path != "/api/v2/me" ||
		request.URL.RawPath != "" || request.URL.Fragment != "" ||
		len(request.URL.RawQuery) > 256 || request.Host != peer.listener.Addr().String() {
		return false
	}
	query := request.URL.Query()
	return len(query) == 1 && len(query["api_key"]) == 1 && query.Get("api_key") == peer.key
}

func (peer *r16ProviderPeer) fail() {
	peer.mu.Lock()
	peer.bad = true
	peer.mu.Unlock()
}

func (peer *r16ProviderPeer) calls() int {
	peer.mu.Lock()
	defer peer.mu.Unlock()
	return peer.requests
}
