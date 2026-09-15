package servertransport

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"io"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/MikeO7/kinosail/packages/privatefile"
)

var errInjected = errors.New("injected TLS failure")

func TestLoadOrCreateRejectsCertificateWithoutPrivateKey(t *testing.T) {
	directory := t.TempDir()
	certificate, _ := testCertificate(t, false)
	var contents bytes.Buffer
	if err := pem.Encode(&contents, &pem.Block{Type: "CERTIFICATE", Bytes: certificate.Raw}); err != nil {
		t.Fatal(err)
	}
	if err := privatefile.Write(filepath.Join(directory, "tls.pem"), contents.Bytes()); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadOrCreate(directory); err == nil {
		t.Fatal("certificate without a private key was accepted")
	}
}

func TestCertificateInputAndDirectoryFailures(t *testing.T) {
	if validLabel("") || validLabel("a_b") {
		t.Fatal("invalid middle label character was accepted")
	}
	blocked := filepath.Join(t.TempDir(), "blocked")
	if err := os.WriteFile(blocked, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := serverCertificateWith(filepath.Join(blocked, "tls"), nil, defaultTLSOperations()); err == nil {
		t.Fatal("blocked TLS directory was accepted")
	}
	operations := defaultTLSOperations()
	operations.mkdirAll = func(string, os.FileMode) error { return errInjected }
	if _, err := serverCertificateWith(t.TempDir(), nil, operations); !errors.Is(err, errInjected) {
		t.Fatalf("directory creation error = %v", err)
	}
	operations = defaultTLSOperations()
	operations.readPrivate = func(string, int64) ([]byte, error) { return nil, errInjected }
	if _, err := loadOrCreate(t.TempDir(), nil, operations); !errors.Is(err, errInjected) {
		t.Fatalf("identity read error = %v", err)
	}
}

func TestLocalAuthorityReportsEveryDependencyFailure(t *testing.T) {
	for name, fail := range map[string]func(*tlsOperations){
		"directory": func(operations *tlsOperations) {
			operations.mkdirAll = func(string, os.FileMode) error { return errInjected }
		},
		"key": func(operations *tlsOperations) {
			operations.generateKey = func() (*ecdsa.PrivateKey, error) { return nil, errInjected }
		},
		"serial": func(operations *tlsOperations) {
			operations.serialNumber = func() (*big.Int, error) { return nil, errInjected }
		},
		"certificate": func(operations *tlsOperations) {
			operations.createCertificate = func(*x509.Certificate, *x509.Certificate, *ecdsa.PublicKey, *ecdsa.PrivateKey) ([]byte, error) {
				return nil, errInjected
			}
		},
		"write": func(operations *tlsOperations) {
			operations.writePrivate = func(string, []byte) error { return errInjected }
		},
	} {
		t.Run(name, func(t *testing.T) {
			operations := defaultTLSOperations()
			fail(&operations)
			if _, _, err := localCertificateAuthorityWith(t.TempDir(), operations); !errors.Is(err, errInjected) {
				t.Fatalf("authority error = %v", err)
			}
		})
	}
	operations := defaultTLSOperations()
	operations.createCertificate = func(*x509.Certificate, *x509.Certificate, *ecdsa.PublicKey, *ecdsa.PrivateKey) ([]byte, error) {
		return []byte("invalid certificate"), nil
	}
	if _, _, err := localCertificateAuthorityWith(t.TempDir(), operations); err == nil {
		t.Fatal("invalid generated authority was accepted")
	}
}

func TestOptionalAuthorityRejectsMalformedAndNonCAIdentity(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "tls-ca.pem")
	if err := privatefile.Write(path, []byte("not PEM")); err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := optionalAuthority(path); err == nil {
		t.Fatal("malformed authority was accepted")
	}
	certificate, key := testCertificate(t, false)
	if err := writeIdentityWith(path, [][]byte{certificate.Raw}, key, defaultTLSOperations()); err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := optionalAuthority(path); err == nil {
		t.Fatal("non-CA authority was accepted")
	}
}

func TestIssueCertificateReportsEveryDependencyFailure(t *testing.T) {
	authority, authorityKey, err := localCertificateAuthorityWith(t.TempDir(), defaultTLSOperations())
	if err != nil {
		t.Fatal(err)
	}
	for name, fail := range map[string]func(*tlsOperations){
		"key": func(operations *tlsOperations) {
			operations.generateKey = func() (*ecdsa.PrivateKey, error) { return nil, errInjected }
		},
		"serial": func(operations *tlsOperations) {
			operations.serialNumber = func() (*big.Int, error) { return nil, errInjected }
		},
		"certificate": func(operations *tlsOperations) {
			operations.createCertificate = func(*x509.Certificate, *x509.Certificate, *ecdsa.PublicKey, *ecdsa.PrivateKey) ([]byte, error) {
				return nil, errInjected
			}
		},
		"write": func(operations *tlsOperations) {
			operations.writePrivate = func(string, []byte) error { return errInjected }
		},
	} {
		t.Run(name, func(t *testing.T) {
			operations := defaultTLSOperations()
			fail(&operations)
			if err := issueServerCertificateWith(filepath.Join(t.TempDir(), "tls.pem"), authority, authorityKey, nil, operations); !errors.Is(err, errInjected) {
				t.Fatalf("issue error = %v", err)
			}
		})
	}
}

func TestWriteIdentityReportsMarshalEncodeAndWriteFailures(t *testing.T) {
	_, key := testCertificate(t, false)
	certificate := []byte("certificate")
	for name, fail := range map[string]func(*tlsOperations){
		"marshal": func(operations *tlsOperations) {
			operations.marshalPrivateKey = func(*ecdsa.PrivateKey) ([]byte, error) { return nil, errInjected }
		},
		"certificate PEM": func(operations *tlsOperations) {
			operations.encodePEM = func(io.Writer, *pem.Block) error { return errInjected }
		},
		"private PEM": func(operations *tlsOperations) {
			calls := 0
			operations.encodePEM = func(writer io.Writer, block *pem.Block) error {
				calls++
				if calls == 2 {
					return errInjected
				}
				return pem.Encode(writer, block)
			}
		},
		"write": func(operations *tlsOperations) {
			operations.writePrivate = func(string, []byte) error { return errInjected }
		},
	} {
		t.Run(name, func(t *testing.T) {
			operations := defaultTLSOperations()
			fail(&operations)
			if err := writeIdentityWith(filepath.Join(t.TempDir(), "tls.pem"), [][]byte{certificate}, key, operations); !errors.Is(err, errInjected) {
				t.Fatalf("write identity error = %v", err)
			}
		})
	}
}

func TestTrustAnchorReportsReadParseAuthorityAndEncodeFailures(t *testing.T) {
	if err := WriteTrustAnchor(io.Discard, ""); err == nil {
		t.Fatal("invalid trust-anchor directory was accepted")
	}
	directory := t.TempDir()
	if _, err := LoadOrCreate(directory); err != nil {
		t.Fatal(err)
	}
	operations := defaultTLSOperations()
	operations.readPrivate = func(string, int64) ([]byte, error) { return nil, errInjected }
	if err := writeTrustAnchor(io.Discard, directory, operations); !errors.Is(err, errInjected) {
		t.Fatalf("read error = %v", err)
	}
	operations = defaultTLSOperations()
	operations.readPrivate = func(string, int64) ([]byte, error) {
		return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: []byte("invalid")}), nil
	}
	if err := writeTrustAnchor(io.Discard, directory, operations); err == nil {
		t.Fatal("invalid trust anchor was accepted")
	}
	operations = defaultTLSOperations()
	operations.readPrivate = func(path string, maximum int64) ([]byte, error) {
		contents, err := privatefile.Read(path, maximum)
		_ = os.Remove(filepath.Join(directory, "tls-ca.pem"))
		return contents, err
	}
	operations.generateKey = func() (*ecdsa.PrivateKey, error) { return nil, errInjected }
	if err := writeTrustAnchor(io.Discard, directory, operations); !errors.Is(err, errInjected) {
		t.Fatalf("authority error = %v", err)
	}
	if _, err := LoadOrCreate(directory); err != nil {
		t.Fatal(err)
	}
	operations = defaultTLSOperations()
	operations.encodePEM = func(io.Writer, *pem.Block) error { return errInjected }
	if err := writeTrustAnchor(io.Discard, directory, operations); !errors.Is(err, errInjected) {
		t.Fatalf("encode error = %v", err)
	}
}

func TestFirstCertificateRejectsInvalidDER(t *testing.T) {
	if _, err := firstCertificate([]byte("not PEM")); err == nil {
		t.Fatal("missing certificate block was accepted")
	}
	contents := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: []byte("invalid")})
	if _, err := firstCertificate(contents); err == nil {
		t.Fatal("invalid DER certificate was accepted")
	}
}

func testCertificate(t *testing.T, authority bool) (*x509.Certificate, *ecdsa.PrivateKey) {
	t.Helper()
	operations := defaultTLSOperations()
	key, err := operations.generateKey()
	if err != nil {
		t.Fatal(err)
	}
	template := &x509.Certificate{
		SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "Test"}, NotBefore: time.Now().Add(-time.Minute),
		NotAfter: time.Now().Add(time.Hour), IsCA: authority, BasicConstraintsValid: true,
	}
	if authority {
		template.KeyUsage = x509.KeyUsageCertSign
	}
	der, err := operations.createCertificate(template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	certificate, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	return certificate, key
}
