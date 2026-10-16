package publicgateway

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"slices"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/acme"
	"golang.org/x/sync/singleflight"
)

const (
	certificateCacheLifetime = 10 * time.Minute
	certificateRenewalMargin = time.Hour
	certificateFetchTimeout  = 30 * time.Second
)

var errCertificateUnavailable = errors.New("public certificate is unavailable")

type cachedCertificate struct {
	certificate *tls.Certificate
	until       time.Time
}

type certificateCache struct {
	hostname string
	client   *http.Client
	mu       sync.Mutex
	entries  map[bool]cachedCertificate
	warned   time.Time
	flights  singleflight.Group
}

func CertificateClient(hostname, path string) func(*tls.ClientHelloInfo) (*tls.Certificate, error) {
	cache := &certificateCache{
		hostname: hostname,
		client:   &http.Client{Transport: unixTransport(path), Timeout: certificateFetchTimeout, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }},
		entries:  make(map[bool]cachedCertificate, 2),
	}
	return cache.certificate
}

func (cache *certificateCache) certificate(hello *tls.ClientHelloInfo) (*tls.Certificate, error) {
	if hello.ServerName != cache.hostname {
		return nil, errors.New("public TLS name is not allowed")
	}
	input := certificateRequest{Name: cache.hostname, ACME: slices.Contains(hello.SupportedProtos, acme.ALPNProto), ECDSA: ecdsaCapable(hello)}
	if input.ACME {
		return cache.fetch(handshakeContext(hello), input)
	}
	cache.mu.Lock()
	entry, found := cache.entries[input.ECDSA]
	cache.mu.Unlock()
	if found && time.Now().Before(entry.until) {
		return entry.certificate, nil
	}
	flight := "rsa"
	if input.ECDSA {
		flight = "ecdsa"
	}
	result := cache.flights.DoChan(flight, func() (any, error) {
		ctx, cancel := context.WithTimeout(context.Background(), certificateFetchTimeout)
		defer cancel()
		certificate, err := cache.fetch(ctx, input)
		if err == nil {
			cache.store(input.ECDSA, certificate)
		}
		return certificate, err
	})
	select {
	case outcome := <-result:
		if outcome.Err != nil {
			return nil, outcome.Err
		}
		return outcome.Val.(*tls.Certificate), nil
	case <-handshakeContext(hello).Done():
		return nil, errCertificateUnavailable
	}
}

func (cache *certificateCache) store(ecdsa bool, certificate *tls.Certificate) {
	if certificate.Leaf == nil {
		return
	}
	now := time.Now()
	until := min(now.Add(certificateCacheLifetime).UnixNano(), certificate.Leaf.NotAfter.Add(-certificateRenewalMargin).UnixNano())
	cache.mu.Lock()
	defer cache.mu.Unlock()
	if until > now.UnixNano() {
		cache.entries[ecdsa] = cachedCertificate{certificate: certificate, until: time.Unix(0, until)}
		return
	}
	if !certificate.Leaf.NotAfter.Equal(cache.warned) {
		cache.warned = certificate.Leaf.NotAfter
		slog.Warn("public certificate expires soon; gateway fetches it on every handshake", "expires_in_minutes", int(time.Until(certificate.Leaf.NotAfter).Minutes()))
	}
}

func (cache *certificateCache) fetch(ctx context.Context, input certificateRequest) (*tls.Certificate, error) {
	body, _ := json.Marshal(input)
	r, err := http.NewRequestWithContext(ctx, http.MethodPost, "http://certificate/", bytes.NewReader(body))
	if err != nil {
		return nil, errCertificateUnavailable
	}
	r.Header.Set("Content-Type", "application/json")
	response, err := cache.client.Do(r)
	if err != nil {
		return nil, errCertificateUnavailable
	}
	defer response.Body.Close()
	data, err := readBounded(response.Body, 256<<10)
	if err != nil || response.StatusCode != http.StatusOK || !strings.HasPrefix(response.Header.Get("Content-Type"), "application/x-pem-file") {
		return nil, errCertificateUnavailable
	}
	certificate, err := tls.X509KeyPair(data, data)
	if err != nil {
		return nil, errCertificateUnavailable
	}
	return &certificate, nil
}

func handshakeContext(hello *tls.ClientHelloInfo) context.Context {
	if ctx := hello.Context(); ctx != nil {
		return ctx
	}
	return context.Background()
}
