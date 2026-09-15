package trustedhttps

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"io"
	"path/filepath"
	"testing"
	"time"

	"golang.org/x/crypto/acme"
)

func TestIdentityOperationsSurfaceCryptographicAndPersistenceFailures(t *testing.T) {
	client := newFakeACME(t, time.Now())
	order := &acme.Order{URI: "order"}
	operations := defaultIdentityOperations()
	operations.generate = func() (*ecdsa.PrivateKey, error) { return nil, errors.New("entropy failed") }
	if _, err := finalizeCertificateWith(t.Context(), client, order, validIssuerConfig().Hostname(), operations, time.Now()); err == nil {
		t.Fatal("certificate key generation failure accepted")
	}
	operations = defaultIdentityOperations()
	operations.createRequest = func(string, crypto.Signer) ([]byte, error) { return nil, errors.New("CSR failed") }
	if _, err := finalizeCertificateWith(t.Context(), client, order, validIssuerConfig().Hostname(), operations, time.Now()); err == nil {
		t.Fatal("certificate request failure accepted")
	}
}

func TestIdentityOperationsSurfaceAccountAndEncodingFailures(t *testing.T) {
	path := filepath.Join(t.TempDir(), "account.pem")
	for name, mutate := range map[string]func(*identityOperations){
		"generate": func(operations *identityOperations) {
			operations.generate = func() (*ecdsa.PrivateKey, error) { return nil, errors.New("entropy failed") }
		},
		"marshal": func(operations *identityOperations) {
			operations.marshal = func(crypto.Signer) ([]byte, error) { return nil, errors.New("marshal failed") }
		},
		"write": func(operations *identityOperations) {
			operations.write = func(string, []byte) error { return errors.New("write failed") }
		},
	} {
		t.Run(name, func(t *testing.T) {
			operations := defaultIdentityOperations()
			mutate(&operations)
			if _, err := accountKeyWith(path+name, operations); err == nil {
				t.Fatal("account key failure accepted")
			}
		})
	}

	identity := testIdentity(t, validIssuerConfig().Hostname(), time.Now().Add(-time.Hour), time.Now().Add(time.Hour))
	certificate, err := tls.X509KeyPair(identity, identity)
	if err != nil {
		t.Fatal(err)
	}
	chain := [][]byte{certificate.Certificate[0]}
	key := certificate.PrivateKey.(crypto.Signer)
	if _, err = encodeIdentityWith([][]byte{[]byte("invalid")}, key, validIssuerConfig().Hostname(), time.Now(), defaultIdentityOperations()); err == nil {
		t.Fatal("invalid leaf encoding accepted")
	}
	operations := defaultIdentityOperations()
	operations.encode = func(io.Writer, *pem.Block) error { return errors.New("encode failed") }
	if _, err = encodeIdentityWith(chain, key, validIssuerConfig().Hostname(), time.Now(), operations); err == nil {
		t.Fatal("certificate PEM failure accepted")
	}
	operations = defaultIdentityOperations()
	operations.marshal = func(crypto.Signer) ([]byte, error) { return nil, errors.New("marshal failed") }
	if _, err = encodeIdentityWith(chain, key, validIssuerConfig().Hostname(), time.Now(), operations); err == nil {
		t.Fatal("private key marshal failure accepted")
	}
	operations = defaultIdentityOperations()
	encodeCalls := 0
	operations.encode = func(writer io.Writer, block *pem.Block) error {
		encodeCalls++
		if encodeCalls == 2 {
			return errors.New("key encode failed")
		}
		return pem.Encode(writer, block)
	}
	if _, err = encodeIdentityWith(chain, key, validIssuerConfig().Hostname(), time.Now(), operations); err == nil {
		t.Fatal("private key PEM failure accepted")
	}
	if _, err = encodeIdentityWith(append(chain, []byte("invalid")), key, validIssuerConfig().Hostname(), time.Now(), defaultIdentityOperations()); err == nil {
		t.Fatal("invalid certificate chain accepted")
	}
}

func TestIdentityValidityAndChainLimitsAreExact(t *testing.T) {
	now := time.Now()
	active := &x509.Certificate{DNSNames: []string{"family.duckdns.org"}, NotBefore: now.Add(-time.Hour), NotAfter: now.Add(time.Hour)}
	if !validIdentityLeaf(active, "family.duckdns.org", now) {
		t.Fatal("active matching certificate rejected")
	}
	for name, leaf := range map[string]*x509.Certificate{
		"hostname": {DNSNames: []string{"other.example"}, NotBefore: now.Add(-time.Hour), NotAfter: now.Add(time.Hour)},
		"future":   {DNSNames: []string{"family.duckdns.org"}, NotBefore: now.Add(time.Hour), NotAfter: now.Add(2 * time.Hour)},
		"expired":  {DNSNames: []string{"family.duckdns.org"}, NotBefore: now.Add(-2 * time.Hour), NotAfter: now.Add(-time.Hour)},
	} {
		if validIdentityLeaf(leaf, "family.duckdns.org", now) {
			t.Fatalf("%s certificate accepted", name)
		}
	}
	if validCertificateChain(nil) || validCertificateChain(make([][]byte, 11)) {
		t.Fatal("invalid chain cardinality accepted")
	}
	if !validCertificateChain(make([][]byte, 10)) || !validCertificateChain([][]byte{make([]byte, 1<<20)}) || validCertificateChain([][]byte{make([]byte, (1<<20)+1)}) {
		t.Fatal("certificate chain size boundary is not exact")
	}
}
