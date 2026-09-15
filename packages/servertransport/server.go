package servertransport

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"time"
)

var secureTLS12CipherSuites = []uint16{
	tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256, tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
	tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384, tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
	tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305_SHA256, tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305_SHA256,
}

// CertificateProvider selects an optional trusted certificate by server name.
type CertificateProvider interface {
	Certificate(string) *tls.Certificate
}

// TLSConfig contains the validated app settings required to serve HTTPS.
type TLSConfig struct {
	Enabled      bool
	DataDir      string
	Hosts        []string
	Certificates CertificateProvider
}

// NewServer returns an HTTP server with Kinosail's shared transport limits.
func NewServer(address string, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:                address,
		Handler:             handler,
		ErrorLog:            privateHTTPErrorLog(),
		TLSConfig:           &tls.Config{MinVersion: tls.VersionTLS12, MaxVersion: tls.VersionTLS13, CipherSuites: secureTLS12CipherSuites},
		ReadHeaderTimeout:   5 * time.Second,
		ReadTimeout:         15 * time.Second,
		IdleTimeout:         2 * time.Minute,
		MaxHeaderBytes:      64 << 10,
		MaxHeaderValueCount: 64,
		HTTP2: &http.HTTP2Config{
			MaxConcurrentStreams:          32,
			MaxDecoderHeaderTableSize:     4096,
			MaxEncoderHeaderTableSize:     4096,
			MaxReadFrameSize:              16 << 10,
			MaxReceiveBufferPerConnection: 1 << 20,
			MaxReceiveBufferPerStream:     256 << 10,
			SendPingTimeout:               30 * time.Second,
			PingTimeout:                   10 * time.Second,
			WriteByteTimeout:              30 * time.Second,
		},
	}
}

// Serve starts the server with plain HTTP or the configured certificate policy.
func Serve(server *http.Server, config TLSConfig) error {
	return serve(server, config, func() error { return server.ListenAndServe() }, func() error { return server.ListenAndServeTLS("", "") })
}

func serve(server *http.Server, config TLSConfig, plain, secure func() error) error {
	if !config.Enabled {
		return plain()
	}
	if err := ConfigureCertificates(server, config); err != nil {
		return err
	}
	return secure()
}

// ConfigureCertificates loads the local identity and installs trusted SNI selection.
func ConfigureCertificates(server *http.Server, config TLSConfig) error {
	certificate, err := LoadOrCreate(config.DataDir, config.Hosts...)
	if err != nil {
		return err
	}
	if server.TLSConfig == nil {
		server.TLSConfig = &tls.Config{MinVersion: tls.VersionTLS12}
	}
	server.TLSConfig.Certificates = []tls.Certificate{certificate}
	server.TLSConfig.GetCertificate = func(hello *tls.ClientHelloInfo) (*tls.Certificate, error) {
		if config.Certificates != nil {
			if selected := config.Certificates.Certificate(hello.ServerName); selected != nil {
				return selected, nil
			}
		}
		return &server.TLSConfig.Certificates[0], nil
	}
	return nil
}

// CheckHealth verifies the loopback health endpoint over the configured protocol.
func CheckHealth(listen string, tlsEnabled bool, authURL string) error {
	_, port, err := net.SplitHostPort(listen)
	if err != nil {
		return err
	}
	scheme := "http"
	transport := http.DefaultTransport
	if tlsEnabled {
		scheme = "https"
		transport = &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true, MinVersion: tls.VersionTLS13, MaxVersion: tls.VersionTLS13}} //nolint:gosec // Loopback health check verifies transport, not PKI trust.
	}
	client := &http.Client{Timeout: 4 * time.Second, Transport: transport}
	request, err := http.NewRequestWithContext(context.Background(), http.MethodGet, scheme+"://127.0.0.1:"+port+"/healthz", nil)
	if err != nil {
		return err
	}
	if authURL != "" {
		origin, parseErr := url.Parse(authURL)
		if parseErr != nil || origin.Host == "" {
			return errors.New("health check auth URL is invalid")
		}
		request.Host = origin.Host
	}
	response, err := client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("health check returned HTTP %d", response.StatusCode)
	}
	return nil
}
