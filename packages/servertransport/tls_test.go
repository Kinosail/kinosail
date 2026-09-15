package servertransport

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestGeneratedIdentityPersistsAndRotatesForChangedHosts(t *testing.T) { //nolint:cyclop // One lifecycle verifies creation, persistence, rotation, and trust export.
	directory := t.TempDir()
	first, err := LoadOrCreate(directory, "server.nox", "192.168.1.10")
	if err != nil {
		t.Fatal(err)
	}
	leaf, err := x509.ParseCertificate(first.Certificate[0])
	if err != nil || leaf.VerifyHostname("server.nox") != nil || leaf.VerifyHostname("192.168.1.10") != nil || len(first.Certificate) != 2 {
		t.Fatalf("generated leaf = %#v, %v", leaf, err)
	}
	before, err := os.ReadFile(filepath.Join(directory, "tls.pem"))
	if err != nil {
		t.Fatal(err)
	}
	repeated, err := LoadOrCreate(directory, "server.nox", "192.168.1.10")
	after, readErr := os.ReadFile(filepath.Join(directory, "tls.pem"))
	if err != nil || readErr != nil || !bytes.Equal(before, after) || !slices.EqualFunc(first.Certificate, repeated.Certificate, bytes.Equal) {
		t.Fatalf("persisted identity changed: %v, %v", err, readErr)
	}
	rotated, err := LoadOrCreate(directory, "other.nox")
	if err != nil {
		t.Fatal(err)
	}
	rotatedLeaf, err := x509.ParseCertificate(rotated.Certificate[0])
	if err != nil || rotatedLeaf.VerifyHostname("other.nox") != nil || bytes.Equal(first.Certificate[0], rotated.Certificate[0]) || !bytes.Equal(first.Certificate[1], rotated.Certificate[1]) {
		t.Fatalf("rotated identity = %#v, %v", rotatedLeaf, err)
	}
	var anchor bytes.Buffer
	if err := WriteTrustAnchor(&anchor, directory); err != nil {
		t.Fatal(err)
	}
	block, rest := pem.Decode(anchor.Bytes())
	authority, err := x509.ParseCertificate(block.Bytes)
	if err != nil || block.Type != "CERTIFICATE" || len(rest) != 0 || !authority.IsCA {
		t.Fatalf("trust anchor = %#v, %d, %v", authority, len(rest), err)
	}
}

func TestOwnerIdentityIsPreservedAndExported(t *testing.T) {
	directory := t.TempDir()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	template := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "Owner supplied"}, NotBefore: time.Now().Add(-time.Minute), NotAfter: time.Now().Add(24 * time.Hour), DNSNames: []string{"owner.example"}}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(directory, "tls.pem")
	if err = writeIdentityWith(path, [][]byte{der}, key, defaultTLSOperations()); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(path)
	if _, err = LoadOrCreate(directory, "different.example"); err != nil {
		t.Fatal(err)
	}
	after, _ := os.ReadFile(path)
	var output bytes.Buffer
	if err = WriteTrustAnchor(&output, directory); err != nil || !bytes.Equal(before, after) {
		t.Fatalf("custom certificate changed: %v", err)
	}
	certificate, err := firstCertificate(output.Bytes())
	if err != nil || certificate.Subject.CommonName != "Owner supplied" {
		t.Fatalf("exported certificate = %#v, %v", certificate, err)
	}
}

func makeIdentityPublic(t *testing.T, path string) {
	t.Helper()
	if err := os.Chmod(path, 0o644); err != nil { //nolint:gosec // The test needs deliberately public key material.
		t.Fatal(err)
	}
}

func makeIdentitySymlink(t *testing.T, path string) {
	t.Helper()
	target := path + ".target"
	if err := os.Rename(path, target); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, path); err != nil {
		t.Fatal(err)
	}
}

func makeIdentityOversized(t *testing.T, path string) {
	t.Helper()
	if err := os.WriteFile(path, make([]byte, (256<<10)+1), 0o600); err != nil {
		t.Fatal(err)
	}
}

func assertUnsafeIdentityRejected(t *testing.T, filename string, mutate func(*testing.T, string)) {
	t.Helper()
	directory := t.TempDir()
	if _, err := LoadOrCreate(directory, "server.nox"); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(directory, filename)
	mutate(t, path)
	before, readErr := os.ReadFile(path)
	infoBefore, statErr := os.Lstat(path)
	leafBefore, leafErr := os.ReadFile(filepath.Join(directory, "tls.pem"))
	if readErr != nil || statErr != nil || leafErr != nil {
		t.Fatal(readErr, statErr, leafErr)
	}
	if _, err := LoadOrCreate(directory, "rotated.example"); err == nil {
		t.Fatal("unsafe private identity was accepted")
	}
	after, _ := os.ReadFile(path)
	infoAfter, _ := os.Lstat(path)
	leafAfter, _ := os.ReadFile(filepath.Join(directory, "tls.pem"))
	if !bytes.Equal(before, after) || infoBefore.Mode() != infoAfter.Mode() || !bytes.Equal(leafBefore, leafAfter) {
		t.Fatal("rejected private identity changed files")
	}
}

func TestIdentityRejectsUnsafePrivateFilesWithoutMutation(t *testing.T) {
	tests := []struct {
		name, filename string
		mutate         func(*testing.T, string)
	}{
		{"leaf public", "tls.pem", makeIdentityPublic},
		{"leaf symlink", "tls.pem", makeIdentitySymlink},
		{"leaf oversized", "tls.pem", makeIdentityOversized},
		{"authority public", "tls-ca.pem", makeIdentityPublic},
		{"authority symlink", "tls-ca.pem", makeIdentitySymlink},
		{"authority oversized", "tls-ca.pem", makeIdentityOversized},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) { assertUnsafeIdentityRejected(t, test.filename, test.mutate) })
	}
}

func TestIdentityRejectsInvalidInputsBeforeEffects(t *testing.T) { //nolint:gocognit // Table covers each bounded identity input class.
	for _, dataDir := range []string{"", strings.Repeat("x", 4097), "invalid\x00path"} {
		if _, err := LoadOrCreate(dataDir); err == nil {
			t.Fatal("invalid data directory was accepted")
		}
	}
	parent := t.TempDir()
	invalid := map[string][]string{
		"empty host":     {""},
		"leading dot":    {".example"},
		"trailing dot":   {"example."},
		"invalid label":  {"bad_label.example"},
		"oversized host": {strings.Repeat("x", 254)},
		"too many hosts": make([]string, 65),
	}
	for name, hosts := range invalid {
		t.Run(name, func(t *testing.T) {
			if name == "too many hosts" {
				for index := range hosts {
					hosts[index] = "server.nox"
				}
			}
			targetDir := filepath.Join(parent, strings.ReplaceAll(name, " ", "-"))
			if _, err := LoadOrCreate(targetDir, hosts...); err == nil {
				t.Fatal("invalid identity input was accepted")
			}
			if _, err := os.Lstat(targetDir); !os.IsNotExist(err) {
				t.Fatalf("invalid input changed filesystem: %v", err)
			}
		})
	}
}

func TestConfigureCertificatesSelectsTrustedSNIAndLocalFallback(t *testing.T) {
	local, trusted := t.TempDir(), t.TempDir()
	trustedPair, err := LoadOrCreate(trusted, "family.example")
	if err != nil {
		t.Fatal(err)
	}
	server := NewServer("", nil)
	provider := fixedProvider{host: "family.example", certificate: &trustedPair}
	if err := ConfigureCertificates(server, TLSConfig{DataDir: local, Certificates: provider}); err != nil {
		t.Fatal(err)
	}
	selected, err := server.TLSConfig.GetCertificate(&tls.ClientHelloInfo{ServerName: "family.example"})
	fallback, fallbackErr := server.TLSConfig.GetCertificate(&tls.ClientHelloInfo{ServerName: "other.example"})
	if err != nil || fallbackErr != nil || selected != &trustedPair || fallback == selected {
		t.Fatalf("certificate selection = %#v, %#v, %v, %v", selected, fallback, err, fallbackErr)
	}
}

type fixedProvider struct {
	host        string
	certificate *tls.Certificate
}

func (provider fixedProvider) Certificate(host string) *tls.Certificate {
	if host == provider.host {
		return provider.certificate
	}
	return nil
}
