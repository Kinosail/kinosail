package federation

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"io"
	"math/big"
	"os"
	"path/filepath"
	"time"

	"github.com/MikeO7/kinosail/packages/privatefile"
)

func loadOrCreateSAMLKeyPair(dataDir string) (*rsa.PrivateKey, *x509.Certificate, error) {
	return loadOrCreateSAMLKeyPairWith(dataDir, generateSAMLKeyPair, privatefile.Write)
}

func loadOrCreateSAMLKeyPairWith(dataDir string, generate func() (*rsa.PrivateKey, *x509.Certificate, error), write func(string, []byte) error) (*rsa.PrivateKey, *x509.Certificate, error) {
	if dataDir == "" {
		return generate()
	}
	path := filepath.Join(dataDir, "saml_sp.pem")
	data, err := privatefile.Read(path, 64<<10)
	if errors.Is(err, os.ErrNotExist) {
		key, certificate, generateErr := generate()
		if generateErr != nil {
			return nil, nil, generateErr
		}
		data = append(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certificate.Raw}), pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})...)
		if err = write(path, data); err != nil {
			return nil, nil, err
		}
	} else if err != nil {
		return nil, nil, err
	}
	pair, err := tls.X509KeyPair(data, data)
	if err != nil || len(pair.Certificate) != 1 {
		return nil, nil, errors.New("SAML service provider key is invalid")
	}
	key, ok := pair.PrivateKey.(*rsa.PrivateKey)
	certificate, certificateErr := x509.ParseCertificate(pair.Certificate[0])
	if !ok || certificateErr != nil {
		return nil, nil, errors.New("SAML service provider key is invalid")
	}
	return key, certificate, nil
}

func generateSAMLKeyPair() (*rsa.PrivateKey, *x509.Certificate, error) {
	return generateSAMLKeyPairWith(rand.Reader, rsa.GenerateKey, createSAMLCertificate)
}

func generateSAMLKeyPairWith(random io.Reader, generate func(io.Reader, int) (*rsa.PrivateKey, error), create func(io.Reader, *x509.Certificate, *rsa.PrivateKey) ([]byte, error)) (*rsa.PrivateKey, *x509.Certificate, error) {
	key, err := generate(random, 2048)
	if err != nil {
		return nil, nil, err
	}
	now := time.Now()
	template := &x509.Certificate{SerialNumber: new(big.Int).SetBytes([]byte(rand.Text())), Subject: pkix.Name{CommonName: "Kinosail SAML"}, NotBefore: now.Add(-time.Hour), NotAfter: now.AddDate(5, 0, 0), KeyUsage: x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}}
	der, err := create(random, template, key)
	if err != nil {
		return nil, nil, err
	}
	certificate, err := x509.ParseCertificate(der)
	return key, certificate, err
}

func createSAMLCertificate(random io.Reader, template *x509.Certificate, key *rsa.PrivateKey) ([]byte, error) {
	return x509.CreateCertificate(random, template, template, &key.PublicKey, key)
}
