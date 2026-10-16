package publicgateway

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"errors"
	"math/big"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"golang.org/x/crypto/acme"
)

func certificateValidFor(t testing.TB, validity time.Duration) tls.Certificate {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	template := &x509.Certificate{SerialNumber: big.NewInt(2), Subject: pkix.Name{CommonName: "family.example"}, DNSNames: []string{"family.example"}, NotBefore: time.Now().Add(-time.Minute), NotAfter: time.Now().Add(validity), KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key}
}

func countedCertificateClient(t *testing.T, issue func(*tls.ClientHelloInfo) (*tls.Certificate, error)) (func(*tls.ClientHelloInfo) (*tls.Certificate, error), *atomic.Int32) {
	t.Helper()
	var calls atomic.Int32
	path := certificateSocket(t, CertificateHandler("family.example", func(hello *tls.ClientHelloInfo) (*tls.Certificate, error) {
		calls.Add(1)
		return issue(hello)
	}))
	return CertificateClient("family.example", path), &calls
}

func modernHello(t *testing.T, protocols ...string) *tls.ClientHelloInfo {
	t.Helper()
	return &tls.ClientHelloInfo{ServerName: "family.example", SupportedVersions: []uint16{tls.VersionTLS13}, SupportedProtos: protocols}
}

func TestCertificateClientReusesIssuedCertificateAcrossHandshakes(t *testing.T) {
	t.Parallel()
	certificate := certificateValidFor(t, 30*24*time.Hour)
	get, calls := countedCertificateClient(t, func(*tls.ClientHelloInfo) (*tls.Certificate, error) { return &certificate, nil })
	var group sync.WaitGroup
	for range 8 {
		group.Go(func() {
			issued, err := get(modernHello(t, "h2", "http/1.1"))
			if err != nil || issued == nil || issued.Leaf == nil || issued.Leaf.SerialNumber.Int64() != 2 {
				t.Errorf("issued certificate = %#v, %v", issued, err)
			}
		})
	}
	group.Wait()
	if _, err := get(modernHello(t, "h2")); err != nil || calls.Load() != 1 {
		t.Fatalf("certificate requests = %d, %v", calls.Load(), err)
	}
	if _, err := get(&tls.ClientHelloInfo{ServerName: "family.example", CipherSuites: []uint16{tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256}}); err != nil || calls.Load() != 2 {
		t.Fatalf("non-ECDSA handshake reused the ECDSA certificate: %d, %v", calls.Load(), err)
	}
	for range 2 {
		if _, err := get(modernHello(t, acme.ALPNProto)); err != nil {
			t.Fatal(err)
		}
	}
	if calls.Load() != 4 {
		t.Fatalf("ACME challenge certificates were cached: %d requests", calls.Load())
	}
}

func TestCertificateClientRefetchesFailuresAndExpiringCertificates(t *testing.T) {
	t.Parallel()
	expiring := certificateValidFor(t, 30*time.Minute)
	fail := atomic.Bool{}
	fail.Store(true)
	get, calls := countedCertificateClient(t, func(*tls.ClientHelloInfo) (*tls.Certificate, error) {
		if fail.Swap(false) {
			return nil, errors.New("issuer unavailable")
		}
		return &expiring, nil
	})
	if _, err := get(modernHello(t, "h2")); err == nil {
		t.Fatal("failed issuance returned a certificate")
	}
	for range 2 {
		if _, err := get(modernHello(t, "h2")); err != nil {
			t.Fatal(err)
		}
	}
	if calls.Load() != 3 {
		t.Fatalf("failure or expiring certificate was cached: %d requests", calls.Load())
	}
}
