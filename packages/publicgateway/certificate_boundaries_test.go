package publicgateway

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"golang.org/x/crypto/acme"
)

func TestCertificateHandlerRejectsUnavailableAndOversizedMaterial(t *testing.T) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name string
		cert *tls.Certificate
		err  error
	}{
		{"error", nil, errors.New("private detail")},
		{"nil", nil, nil},
		{"empty", &tls.Certificate{}, nil},
		{"chain", &tls.Certificate{Certificate: make([][]byte, 9)}, nil},
		{"key", &tls.Certificate{Certificate: [][]byte{{1}}, PrivateKey: struct{}{}}, nil},
		{"large certificate", &tls.Certificate{Certificate: [][]byte{make([]byte, 64<<10+1)}, PrivateKey: key}, nil},
		{"large response", &tls.Certificate{Certificate: [][]byte{make([]byte, 64<<10), make([]byte, 64<<10), make([]byte, 64<<10)}, PrivateKey: key}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			handler := CertificateHandler("family.example", func(*tls.ClientHelloInfo) (*tls.Certificate, error) { return tc.cert, tc.err })
			request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", strings.NewReader(`{"name":"family.example"}`))
			request.Header.Set("Content-Type", "application/json")
			result := httptest.NewRecorder()
			handler.ServeHTTP(result, request)
			if result.Code != http.StatusServiceUnavailable || strings.Contains(result.Body.String(), "private detail") {
				t.Fatalf("response %d %s", result.Code, result.Body.String())
			}
		})
	}
}

func TestCertificateHandlerPreservesNegotiation(t *testing.T) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		acme      bool
		protocols []string
	}{
		{false, []string{"h2", "http/1.1"}}, {true, []string{acme.ALPNProto}},
	} {
		called := false
		handler := CertificateHandler("family.example", func(hello *tls.ClientHelloInfo) (*tls.Certificate, error) {
			called = true
			if !ecdsaCapable(hello) {
				t.Error("lost ECDSA negotiation")
			}
			if !slices.Equal(hello.SupportedProtos, tc.protocols) {
				t.Error("lost protocol negotiation")
			}
			return &tls.Certificate{Certificate: [][]byte{{1}}, PrivateKey: key}, nil
		})
		body, err := json.Marshal(certificateRequest{Name: "family.example", ECDSA: true, ACME: tc.acme})
		if err != nil {
			t.Fatal(err)
		}
		request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", bytes.NewReader(body))
		request.Header.Set("Content-Type", "application/json")
		result := httptest.NewRecorder()
		handler.ServeHTTP(result, request)
		if !called || result.Code != http.StatusOK || result.Header().Get("Cache-Control") != "no-store" || result.Header().Get("Content-Type") != "application/x-pem-file" {
			t.Fatalf("response %d", result.Code)
		}
	}
}

func TestECDSACapabilities(t *testing.T) {
	for _, tc := range []struct {
		hello tls.ClientHelloInfo
		want  bool
	}{
		{tls.ClientHelloInfo{}, false},
		{tls.ClientHelloInfo{SignatureSchemes: []tls.SignatureScheme{tls.PSSWithSHA256}, SupportedVersions: []uint16{tls.VersionTLS13}}, false},
		{tls.ClientHelloInfo{SupportedCurves: []tls.CurveID{tls.X25519}, SupportedVersions: []uint16{tls.VersionTLS13}}, false},
		{tls.ClientHelloInfo{SupportedVersions: []uint16{tls.VersionTLS13}}, true},
		{tls.ClientHelloInfo{CipherSuites: []uint16{tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256}}, true},
		{tls.ClientHelloInfo{CipherSuites: []uint16{tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384}}, true},
		{tls.ClientHelloInfo{CipherSuites: []uint16{tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305_SHA256}}, true},
	} {
		if got := ecdsaCapable(&tc.hello); got != tc.want {
			t.Errorf("capability=%v want %v for %+v", got, tc.want, tc.hello)
		}
	}
}
