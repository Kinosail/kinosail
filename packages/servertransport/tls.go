// Package servertransport owns Kinosail's hardened HTTP and local TLS boundary.
package servertransport

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/MikeO7/kinosail/packages/privatefile"
)

// LoadOrCreate returns an owner identity or a current Kinosail local identity.
func LoadOrCreate(dataDir string, hosts ...string) (tls.Certificate, error) {
	return loadOrCreate(dataDir, hosts, defaultTLSOperations())
}

func loadOrCreate(dataDir string, hosts []string, operations tlsOperations) (tls.Certificate, error) {
	path, err := serverCertificateWith(dataDir, hosts, operations)
	if err != nil {
		return tls.Certificate{}, err
	}
	contents, err := operations.readPrivate(path, 256<<10)
	if err != nil {
		return tls.Certificate{}, err
	}
	return tls.X509KeyPair(contents, contents)
}

func serverCertificateWith(dataDir string, hosts []string, operations tlsOperations) (string, error) { //nolint:cyclop // Owner and generated identity paths must fail closed in one lifecycle.
	if err := validateIdentityInput(dataDir, hosts); err != nil {
		return "", err
	}
	path := filepath.Join(dataDir, "tls.pem")
	certificate, exists, err := optionalCertificate(path)
	if err != nil {
		return "", err
	}
	if exists && !generatedCertificate(certificate) {
		return path, nil
	}
	if err := operations.mkdirAll(dataDir, 0o700); err != nil {
		return "", err
	}
	authority, authorityKey, err := localCertificateAuthorityWith(dataDir, operations)
	if err != nil {
		return "", err
	}
	if exists && certificate.CheckSignatureFrom(authority) == nil && certificateMatches(certificate, hosts) && time.Until(certificate.NotAfter) > 30*24*time.Hour {
		return path, nil
	}
	return path, issueServerCertificateWith(path, authority, authorityKey, hosts, operations)
}

func validateIdentityInput(dataDir string, hosts []string) error {
	if dataDir == "" || len(dataDir) > 4096 || strings.ContainsRune(dataDir, 0) || len(hosts) > 64 {
		return errors.New("invalid local TLS configuration")
	}
	for _, host := range hosts {
		if !validHost(host) {
			return errors.New("invalid local TLS host")
		}
	}
	return nil
}

func validHost(host string) bool {
	if net.ParseIP(host) != nil {
		return true
	}
	if host == "" || len(host) > 253 || strings.HasPrefix(host, ".") || strings.HasSuffix(host, ".") {
		return false
	}
	for label := range strings.SplitSeq(host, ".") {
		if !validLabel(label) {
			return false
		}
	}
	return true
}

func validLabel(label string) bool {
	if label == "" || len(label) > 63 || !letterOrDigit(label[0]) || !letterOrDigit(label[len(label)-1]) {
		return false
	}
	for index := 1; index < len(label)-1; index++ {
		if !letterOrDigit(label[index]) && label[index] != '-' {
			return false
		}
	}
	return true
}

func letterOrDigit(character byte) bool {
	return character >= 'a' && character <= 'z' || character >= 'A' && character <= 'Z' || character >= '0' && character <= '9'
}

func localCertificateAuthorityWith(dataDir string, operations tlsOperations) (*x509.Certificate, *ecdsa.PrivateKey, error) {
	path := filepath.Join(dataDir, "tls-ca.pem")
	certificate, key, exists, err := optionalAuthority(path)
	if err != nil {
		return nil, nil, err
	}
	if exists {
		return certificate, key, nil
	}
	if err := operations.mkdirAll(dataDir, 0o700); err != nil {
		return nil, nil, err
	}
	key, err = operations.generateKey()
	if err != nil {
		return nil, nil, err
	}
	serial, err := operations.serialNumber()
	if err != nil {
		return nil, nil, err
	}
	now := time.Now()
	template := &x509.Certificate{SerialNumber: serial, Subject: pkix.Name{CommonName: "Kinosail Local CA", Organization: []string{"Kinosail generated"}}, NotBefore: now.Add(-5 * time.Minute), NotAfter: now.AddDate(10, 0, 0), KeyUsage: x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign | x509.KeyUsageCRLSign, BasicConstraintsValid: true, IsCA: true, MaxPathLen: 0, MaxPathLenZero: true}
	der, err := operations.createCertificate(template, template, &key.PublicKey, key)
	if err != nil {
		return nil, nil, err
	}
	if err = writeIdentityWith(path, [][]byte{der}, key, operations); err != nil {
		return nil, nil, err
	}
	certificate, err = x509.ParseCertificate(der)
	return certificate, key, err
}

func optionalAuthority(path string) (*x509.Certificate, *ecdsa.PrivateKey, bool, error) {
	contents, err := privatefile.Read(path, 256<<10)
	if os.IsNotExist(err) {
		return nil, nil, false, nil
	}
	if err != nil {
		return nil, nil, false, err
	}
	pair, err := tls.X509KeyPair(contents, contents)
	if err != nil {
		return nil, nil, false, fmt.Errorf("load local TLS authority: %w", err)
	}
	certificate, parseErr := x509.ParseCertificate(pair.Certificate[0])
	key, keyOK := pair.PrivateKey.(*ecdsa.PrivateKey)
	if parseErr != nil || !keyOK || !certificate.IsCA || !generatedCertificate(certificate) {
		return nil, nil, false, errors.New("local TLS authority is invalid")
	}
	return certificate, key, true, nil
}

func issueServerCertificateWith(path string, authority *x509.Certificate, authorityKey *ecdsa.PrivateKey, hosts []string, operations tlsOperations) error {
	key, err := operations.generateKey()
	if err != nil {
		return err
	}
	serial, err := operations.serialNumber()
	if err != nil {
		return err
	}
	now := time.Now()
	template := &x509.Certificate{SerialNumber: serial, Subject: pkix.Name{CommonName: "Kinosail", Organization: []string{"Kinosail generated"}}, NotBefore: now.Add(-5 * time.Minute), NotAfter: now.Add(397*24*time.Hour - 5*time.Minute), KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}, DNSNames: []string{"localhost", "kinosail"}, IPAddresses: []net.IP{net.ParseIP("127.0.0.1"), net.ParseIP("::1")}}
	for _, host := range hosts {
		if address := net.ParseIP(host); address != nil {
			template.IPAddresses = append(template.IPAddresses, address)
		} else if host != "" {
			template.DNSNames = append(template.DNSNames, host)
		}
	}
	certificate, err := operations.createCertificate(template, authority, &key.PublicKey, authorityKey)
	if err != nil {
		return err
	}
	return writeIdentityWith(path, [][]byte{certificate, authority.Raw}, key, operations)
}

func writeIdentityWith(path string, certificates [][]byte, key *ecdsa.PrivateKey, operations tlsOperations) error {
	privateKey, err := operations.marshalPrivateKey(key)
	if err != nil {
		return err
	}
	contents := new(bytes.Buffer)
	for _, certificate := range certificates {
		if err = operations.encodePEM(contents, &pem.Block{Type: "CERTIFICATE", Bytes: certificate}); err != nil {
			return err
		}
	}
	if err = operations.encodePEM(contents, &pem.Block{Type: "PRIVATE KEY", Bytes: privateKey}); err != nil {
		return err
	}
	return operations.writePrivate(path, contents.Bytes())
}

func serialNumber() (*big.Int, error) {
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err == nil {
		serial.Add(serial, big.NewInt(1))
	}
	return serial, err
}

func firstCertificate(contents []byte) (*x509.Certificate, error) {
	block, _ := pem.Decode(contents)
	if block == nil || block.Type != "CERTIFICATE" {
		return nil, errors.New("TLS certificate is invalid")
	}
	return x509.ParseCertificate(block.Bytes)
}

func optionalCertificate(path string) (*x509.Certificate, bool, error) {
	contents, err := privatefile.Read(path, 256<<10)
	if os.IsNotExist(err) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	certificate, err := firstCertificate(contents)
	return certificate, true, err
}

func generatedCertificate(certificate *x509.Certificate) bool {
	return len(certificate.Subject.Organization) == 1 && certificate.Subject.Organization[0] == "Kinosail generated"
}

func certificateMatches(certificate *x509.Certificate, hosts []string) bool {
	for _, host := range hosts {
		if certificate.VerifyHostname(host) != nil {
			return false
		}
	}
	return true
}

// WriteTrustAnchor writes only the public certificate users can install.
func WriteTrustAnchor(writer io.Writer, dataDir string) error {
	return writeTrustAnchor(writer, dataDir, defaultTLSOperations())
}

func writeTrustAnchor(writer io.Writer, dataDir string, operations tlsOperations) error {
	path, err := serverCertificateWith(dataDir, nil, operations)
	if err != nil {
		return err
	}
	contents, err := operations.readPrivate(path, 256<<10)
	if err != nil {
		return err
	}
	certificate, err := firstCertificate(contents)
	if err != nil {
		return err
	}
	if generatedCertificate(certificate) {
		certificate, _, err = localCertificateAuthorityWith(dataDir, operations)
		if err != nil {
			return err
		}
	}
	return operations.encodePEM(writer, &pem.Block{Type: "CERTIFICATE", Bytes: certificate.Raw})
}
