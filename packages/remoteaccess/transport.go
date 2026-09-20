package remoteaccess

import (
	"context"
	"crypto/tls"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golang.org/x/crypto/acme"
	"golang.org/x/net/http2"
)

// Serve updates DuckDNS before opening a TLS-only listener and closes with ctx.
func (manager *Manager) Serve(ctx context.Context, handler http.Handler) error { //nolint:cyclop,funlen,gocognit // Listener startup is intentionally linear and fail-closed.
	if !manager.config.Enabled {
		return nil
	}
	manager.mu.RLock()
	killed := manager.killed
	manager.mu.RUnlock()
	if killed {
		return nil
	}
	for {
		manager.mu.RLock()
		stopped := manager.killed
		manager.mu.RUnlock()
		if stopped || ctx.Err() != nil {
			return nil
		}
		if err := manager.update(ctx); err != nil {
			manager.setStatus("error", err)
			select {
			case <-ctx.Done():
				return nil
			case <-manager.operations.after(time.Minute):
			}
			continue
		}
		break
	}
	if !manager.config.PublicHTTPS {
		manager.setStatus("ready", nil)
		manager.refresh(ctx)
		return nil
	}
	if manager.config.Gateway {
		return manager.serveGateway(ctx, handler)
	}
	server := &http.Server{
		Addr: manager.config.Listen, Handler: exactPublicHost(manager.hostname, handler),
		TLSConfig:         &tls.Config{MinVersion: tls.VersionTLS12, MaxVersion: tls.VersionTLS13, CipherSuites: []uint16{tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256, tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384, tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305_SHA256, tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256, tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384, tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305_SHA256}, GetCertificate: manager.certificate, NextProtos: []string{"h2", "http/1.1", acme.ALPNProto}},
		ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, IdleTimeout: 2 * time.Minute, MaxHeaderBytes: 64 << 10, MaxHeaderValueCount: 64,
		ConnState: manager.trackConnection,
	}
	if err := manager.operations.configureServer(server, &http2.Server{
		MaxConcurrentStreams:         32,
		MaxDecoderHeaderTableSize:    4096,
		MaxEncoderHeaderTableSize:    4096,
		MaxReadFrameSize:             16 << 10,
		IdleTimeout:                  2 * time.Minute,
		ReadIdleTimeout:              30 * time.Second,
		PingTimeout:                  10 * time.Second,
		WriteByteTimeout:             30 * time.Second,
		MaxUploadBufferPerConnection: 1 << 20,
		MaxUploadBufferPerStream:     256 << 10,
	}); err != nil {
		manager.setStatus("error", err)
		return err
	}
	listener, err := manager.operations.listen(ctx, "tcp", manager.config.Listen)
	if err != nil {
		manager.setStatus("error", err)
		return err
	}
	manager.mu.Lock()
	if manager.killed {
		manager.mu.Unlock()
		_ = listener.Close()
		return nil
	}
	manager.server, manager.listener = server, listener
	manager.mu.Unlock()
	defer func() {
		manager.mu.Lock()
		if manager.server == server {
			manager.server, manager.listener = nil, nil
		}
		manager.mu.Unlock()
	}()
	go func() {
		<-ctx.Done()
		_ = server.Close()
	}()
	manager.setStatus("ready", nil)
	refreshContext, stopRefresh := context.WithCancel(ctx)
	defer stopRefresh()
	go manager.refresh(refreshContext)
	err = server.Serve(tls.NewListener(listener, server.TLSConfig))
	manager.mu.RLock()
	killed = manager.killed
	manager.mu.RUnlock()
	if killed || errors.Is(err, http.ErrServerClosed) || errors.Is(err, net.ErrClosed) {
		return nil
	}
	manager.setStatus("error", err)
	return err
}

func exactPublicHost(hostname string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		host := strings.ToLower(strings.TrimSuffix(request.Host, "."))
		if host != hostname && host != hostname+":443" {
			http.Error(writer, "misdirected request", http.StatusMisdirectedRequest)
			return
		}
		if len(request.TransferEncoding) != 0 {
			writer.Header().Set("Connection", "close")
			http.Error(writer, "bad request", http.StatusBadRequest)
			return
		}
		next.ServeHTTP(writer, request)
	})
}

func (manager *Manager) refresh(ctx context.Context) {
	ticks, stop := manager.operations.refreshTicks()
	defer stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticks:
			if err := manager.update(ctx); err != nil {
				manager.setStatus("error", err)
			} else {
				manager.setStatus("ready", nil)
			}
		}
	}
}

func (manager *Manager) update(ctx context.Context) error {
	endpoint, err := url.Parse(manager.updateURL)
	if err != nil {
		return errors.New("create DuckDNS update")
	}
	query := endpoint.Query()
	query.Set("domains", manager.config.Domain)
	query.Set("token", manager.config.Token)
	query.Set("ip", "")
	endpoint.RawQuery = query.Encode()
	request := (&http.Request{Method: http.MethodGet, URL: endpoint, Header: make(http.Header)}).WithContext(ctx)
	response, err := manager.client.Do(request)
	if err != nil {
		return errors.New("DuckDNS update failed")
	}
	defer response.Body.Close()
	body, readErr := io.ReadAll(io.LimitReader(response.Body, 65))
	if readErr != nil || response.StatusCode != http.StatusOK || len(body) > 64 || strings.TrimSpace(string(body)) != "OK" {
		return errors.New("DuckDNS rejected the update")
	}
	manager.mu.Lock()
	manager.status.LastDDNSUpdate = time.Now().UTC().Format(time.RFC3339)
	manager.mu.Unlock()
	return nil
}

func (manager *Manager) trackConnection(connection net.Conn, state http.ConnState) { //nolint:cyclop // Connection accounting and close decisions must remain atomic.
	manager.mu.Lock()
	closeConnection := manager.killed
	switch state {
	case http.StateNew:
		source := remoteHost(connection.RemoteAddr().String())
		sourceLimit := 32
		if manager.config.Gateway && connection.RemoteAddr().Network() == "unix" {
			sourceLimit = 256 // The gateway enforces per-client limits before multiplexing onto this socket.
		}
		if !closeConnection && len(manager.connections) < 256 && manager.sources[source] < sourceLimit {
			manager.connections[connection] = source
			manager.sources[source]++
		} else {
			closeConnection = true
		}
	case http.StateClosed, http.StateHijacked:
		if source, found := manager.connections[connection]; found {
			delete(manager.connections, connection)
			manager.sources[source]--
			if manager.sources[source] == 0 {
				delete(manager.sources, source)
			}
		}
	case http.StateActive, http.StateIdle:
	}
	manager.status.Connections = len(manager.connections)
	manager.mu.Unlock()
	if closeConnection && state != http.StateClosed && state != http.StateHijacked {
		_ = connection.Close()
	}
}

func remoteHost(address string) string {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return address
	}
	return host
}
