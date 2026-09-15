// Package publicgateway is the restricted public HTTPS process. It has no
// application configuration, Owner identity, media mount, or IP dial capability.
package publicgateway

import (
	"context"
	"crypto/tls"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/MikeO7/kinosail/packages/servertransport"
	"golang.org/x/crypto/acme"
)

const Directory = "/public-transport"
const HTTPPath = Directory + "/http.sock"
const CertificatePath = Directory + "/certificate.sock"
const ClientAddressHeader = "X-Kinosail-Public-Client"

// Run opens exactly one public listener before installing a process-wide network
// filter. Failure to install the filter closes the listener without serving.
func Run(ctx context.Context, hostname string) error {
	if !validHostname(hostname) {
		return errors.New("public gateway hostname is invalid")
	}
	listener, err := net.Listen("tcp", ":8443")
	if err != nil {
		return err
	}
	defer listener.Close()
	if err = restrictNetwork(); err != nil {
		return err
	}
	server := servertransport.NewServer(":8443", Handler(hostname, HTTPPath))
	server.TLSConfig.GetCertificate = CertificateClient(hostname, CertificatePath)
	server.TLSConfig.NextProtos = []string{"h2", "http/1.1", acme.ALPNProto}
	var limits connectionLimits
	server.ConnState = limits.track
	go func() { <-ctx.Done(); _ = server.Close() }()
	err = server.ServeTLS(listener, "", "")
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

func unixTransport(path string) *http.Transport {
	return &http.Transport{
		Proxy: nil,
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			return (&net.Dialer{Timeout: 5 * time.Second}).DialContext(ctx, "unix", path)
		},
		MaxIdleConns: 32, MaxIdleConnsPerHost: 32, MaxConnsPerHost: 128, IdleConnTimeout: 30 * time.Second,
		ResponseHeaderTimeout: 30 * time.Second, MaxResponseHeaderBytes: 64 << 10,
	}
}

// Handler forwards requests only to the public application socket. Caller-supplied
// proxy metadata cannot select another target or turn a public request into management.
func Handler(hostname, path string) http.Handler {
	proxy := &httputil.ReverseProxy{
		Rewrite: func(p *httputil.ProxyRequest) {
			p.SetURL(&url.URL{Scheme: "http", Host: "public"})
			p.Out.Host = p.In.Host
			for _, name := range []string{"Forwarded", "X-Forwarded-For", "X-Forwarded-Host", "X-Forwarded-Proto", "X-Kinosail-Proxy-Token", "X-Kinosail-Remote", ClientAddressHeader} {
				p.Out.Header.Del(name)
			}
			p.Out.Header.Set(ClientAddressHeader, p.In.RemoteAddr)
		},
		Transport: unixTransport(path), FlushInterval: -1,
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, _ error) {
			http.Error(w, "public access is unavailable", http.StatusServiceUnavailable)
		},
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.TLS == nil || r.Host != hostname && r.Host != hostname+":443" {
			http.Error(w, "misdirected request", http.StatusMisdirectedRequest)
			return
		}
		if r.Method == http.MethodConnect || len(r.TransferEncoding) > 0 || r.URL.IsAbs() || r.URL.Host != "" || r.ContentLength > 1<<20 {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
		proxy.ServeHTTP(w, r)
	})
}

func validHostname(host string) bool {
	if len(host) > 253 || !strings.Contains(host, ".") || net.ParseIP(host) != nil {
		return false
	}
	for _, label := range strings.Split(host, ".") {
		if label == "" || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
		for _, c := range label {
			if !(c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '-') {
				return false
			}
		}
	}
	return true
}

type connectionLimits struct {
	mu          sync.Mutex
	connections map[net.Conn]string
	sources     map[string]int
}

func (l *connectionLimits) track(c net.Conn, state http.ConnState) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.connections == nil {
		l.connections = map[net.Conn]string{}
		l.sources = map[string]int{}
	}
	switch state {
	case http.StateNew:
		host, _, _ := net.SplitHostPort(c.RemoteAddr().String())
		if len(l.connections) >= 256 || l.sources[host] >= 32 {
			_ = c.Close()
			return
		}
		l.connections[c] = host
		l.sources[host]++
	case http.StateClosed, http.StateHijacked:
		if host, ok := l.connections[c]; ok {
			delete(l.connections, c)
			l.sources[host]--
			if l.sources[host] == 0 {
				delete(l.sources, host)
			}
		}
	}
}

// TLSMetadata is used only on the private socket receiving HTTPS from this process.
// It does not imply identity or Owner permissions.
func TLSMetadata(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		values := r.Header.Values(ClientAddressHeader)
		if len(values) != 1 || len(values[0]) > 256 {
			http.Error(w, "bad gateway request", http.StatusBadRequest)
			return
		}
		host, port, err := net.SplitHostPort(values[0])
		number, portErr := strconv.Atoi(port)
		if err != nil || net.ParseIP(host) == nil || portErr != nil || number < 1 || number > 65535 || strconv.Itoa(number) != port {
			http.Error(w, "bad gateway request", http.StatusBadRequest)
			return
		}
		r.RemoteAddr = values[0]
		r.Header.Del(ClientAddressHeader)
		r.TLS = &tls.ConnectionState{Version: tls.VersionTLS13, HandshakeComplete: true}
		next.ServeHTTP(w, r)
	})
}

func readBounded(reader io.Reader, maximum int64) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(reader, maximum+1))
	if err != nil || int64(len(data)) > maximum {
		return nil, errors.New("gateway response is invalid")
	}
	return data, nil
}
