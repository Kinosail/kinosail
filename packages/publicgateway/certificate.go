package publicgateway

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/MikeO7/kinosail/packages/httpguard"
	"golang.org/x/crypto/acme"
)

type certificateRequest struct {
	Name  string `json:"name"`
	ACME  bool   `json:"acme"`
	ECDSA bool   `json:"ecdsa"`
}

// CertificateHandler exposes only the public TLS identity over a separate private
// socket. It never exports ACME account keys, DNS credentials, or application state.
func CertificateHandler(hostname string, get func(*tls.ClientHelloInfo) (*tls.Certificate, error)) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		if r.Method != http.MethodPost || r.URL.Path != "/" || r.URL.RawQuery != "" || r.URL.ForceQuery || r.ContentLength > 1024 || r.Header.Get("Content-Type") != "application/json" {
			http.Error(w, "invalid certificate request", http.StatusBadRequest)
			return
		}
		var input certificateRequest
		if httpguard.DecodeUniqueJSON(http.MaxBytesReader(w, r.Body, 1024), 1024, &input) != nil || input.Name != hostname {
			http.Error(w, "invalid certificate request", http.StatusBadRequest)
			return
		}
		protocols := []string{"h2", "http/1.1"}
		if input.ACME {
			protocols = []string{acme.ALPNProto}
		}
		hello := &tls.ClientHelloInfo{ServerName: input.Name, SupportedProtos: protocols}
		if input.ECDSA {
			hello.CipherSuites = []uint16{tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256}
			hello.SupportedCurves = []tls.CurveID{tls.CurveP256}
			hello.SignatureSchemes = []tls.SignatureScheme{tls.ECDSAWithP256AndSHA256}
		}
		certificate, err := get(hello)
		if err != nil || certificate == nil || len(certificate.Certificate) == 0 || len(certificate.Certificate) > 8 {
			http.Error(w, "public certificate is unavailable", http.StatusServiceUnavailable)
			return
		}
		key, err := x509.MarshalPKCS8PrivateKey(certificate.PrivateKey)
		if err != nil {
			http.Error(w, "public certificate is unavailable", http.StatusServiceUnavailable)
			return
		}
		body := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: key})
		for _, der := range certificate.Certificate {
			if len(der) > 64<<10 {
				http.Error(w, "public certificate is unavailable", http.StatusServiceUnavailable)
				return
			}
			body = append(body, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})...)
		}
		if len(body) > 256<<10 {
			http.Error(w, "public certificate is unavailable", http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "application/x-pem-file")
		_, _ = w.Write(body)
	})
}

func CertificateClient(hostname, path string) func(*tls.ClientHelloInfo) (*tls.Certificate, error) {
	client := &http.Client{Transport: unixTransport(path), Timeout: 30 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	return func(hello *tls.ClientHelloInfo) (*tls.Certificate, error) {
		if hello.ServerName != hostname {
			return nil, errors.New("public TLS name is not allowed")
		}
		input := certificateRequest{Name: hostname}
		input.ECDSA = ecdsaCapable(hello)
		for _, protocol := range hello.SupportedProtos {
			if protocol == acme.ALPNProto {
				input.ACME = true
			}
		}
		body, _ := json.Marshal(input)
		r, err := http.NewRequestWithContext(hello.Context(), http.MethodPost, "http://certificate/", bytes.NewReader(body))
		if err != nil {
			return nil, errors.New("public certificate is unavailable")
		}
		r.Header.Set("Content-Type", "application/json")
		response, err := client.Do(r)
		if err != nil {
			return nil, errors.New("public certificate is unavailable")
		}
		defer response.Body.Close()
		data, err := readBounded(response.Body, 256<<10)
		if err != nil || response.StatusCode != http.StatusOK || !strings.HasPrefix(response.Header.Get("Content-Type"), "application/x-pem-file") {
			return nil, errors.New("public certificate is unavailable")
		}
		certificate, err := tls.X509KeyPair(data, data)
		if err != nil {
			return nil, errors.New("public certificate is unavailable")
		}
		return &certificate, nil
	}
}

func ecdsaCapable(hello *tls.ClientHelloInfo) bool {
	if hello.SignatureSchemes != nil && !slices.Contains(hello.SignatureSchemes, tls.ECDSAWithP256AndSHA256) {
		return false
	}
	if hello.SupportedCurves != nil && !slices.Contains(hello.SupportedCurves, tls.CurveP256) {
		return false
	}
	if slices.Contains(hello.SupportedVersions, tls.VersionTLS13) {
		return true
	}
	return slices.Contains(hello.CipherSuites, tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256) || slices.Contains(hello.CipherSuites, tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384) || slices.Contains(hello.CipherSuites, tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305_SHA256)
}
