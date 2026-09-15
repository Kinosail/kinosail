package trustedhttps

import (
	"bytes"
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"io"
	"os"
	"time"

	"golang.org/x/crypto/acme"
)

type identityOperations struct {
	generate      func() (*ecdsa.PrivateKey, error)
	createRequest func(string, crypto.Signer) ([]byte, error)
	marshal       func(crypto.Signer) ([]byte, error)
	encode        func(io.Writer, *pem.Block) error
	write         func(string, []byte) error
}

func defaultIdentityOperations() identityOperations {
	return identityOperations{
		generate: func() (*ecdsa.PrivateKey, error) { return ecdsa.GenerateKey(elliptic.P256(), rand.Reader) },
		createRequest: func(hostname string, key crypto.Signer) ([]byte, error) {
			return x509.CreateCertificateRequest(rand.Reader, &x509.CertificateRequest{DNSNames: []string{hostname}}, key)
		},
		marshal: func(key crypto.Signer) ([]byte, error) { return x509.MarshalPKCS8PrivateKey(key) },
		encode:  pem.Encode,
		write:   writePrivate,
	}
}

func finalizeCertificateWith(ctx context.Context, client acmeClient, order *acme.Order, hostname string, operations identityOperations, now time.Time) ([]byte, error) {
	var err error
	order, err = client.WaitOrder(ctx, order.URI)
	if err != nil || !validFinalOrder(order) {
		return nil, errors.New("prepare certificate order")
	}
	key, err := operations.generate()
	if err != nil {
		return nil, errors.New("generate certificate key")
	}
	csr, err := operations.createRequest(hostname, key)
	if err != nil {
		return nil, errors.New("create certificate request")
	}
	chain, _, err := client.CreateOrderCert(ctx, order.FinalizeURL, csr, true)
	if err != nil || !validCertificateChain(chain) {
		return nil, errors.New("issue trusted HTTPS certificate")
	}
	return encodeIdentityWith(chain, key, hostname, now, operations)
}

func accountKey(path string) (*ecdsa.PrivateKey, error) {
	return accountKeyWith(path, defaultIdentityOperations())
}

func accountKeyWith(path string, operations identityOperations) (*ecdsa.PrivateKey, error) {
	contents, err := readBoundedFile(path, 16<<10)
	if err == nil {
		block, _ := pem.Decode(contents)
		if block == nil || block.Type != "PRIVATE KEY" {
			return nil, errors.New("certificate account key is invalid")
		}
		key, parseErr := x509.ParsePKCS8PrivateKey(block.Bytes)
		ecdsaKey, ok := key.(*ecdsa.PrivateKey)
		if parseErr != nil || !ok {
			return nil, errors.New("certificate account key is invalid")
		}
		return ecdsaKey, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return nil, errors.New("read certificate account key")
	}
	key, err := operations.generate()
	if err != nil {
		return nil, errors.New("generate certificate account key")
	}
	encoded, err := operations.marshal(key)
	if err != nil {
		return nil, errors.New("encode certificate account key")
	}
	if err = operations.write(path, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: encoded})); err != nil {
		return nil, errors.New("save certificate account key")
	}
	return key, nil
}

func encodeIdentityWith(chain [][]byte, key crypto.Signer, hostname string, now time.Time, operations identityOperations) ([]byte, error) {
	leaf, err := x509.ParseCertificate(chain[0])
	if err != nil || !validIdentityLeaf(leaf, hostname, now) {
		return nil, errors.New("issued certificate is invalid")
	}
	contents := new(bytes.Buffer)
	for _, der := range chain {
		if _, err = x509.ParseCertificate(der); err != nil {
			return nil, errors.New("issued certificate chain is invalid")
		}
		if err = operations.encode(contents, &pem.Block{Type: "CERTIFICATE", Bytes: der}); err != nil {
			return nil, errors.New("encode trusted HTTPS certificate")
		}
	}
	privateKey, err := operations.marshal(key)
	if err != nil || operations.encode(contents, &pem.Block{Type: "PRIVATE KEY", Bytes: privateKey}) != nil {
		return nil, errors.New("encode trusted HTTPS certificate")
	}
	if _, err = tls.X509KeyPair(contents.Bytes(), contents.Bytes()); err != nil {
		return nil, errors.New("issued certificate key does not match")
	}
	return contents.Bytes(), nil
}

func validFinalOrder(order *acme.Order) bool {
	return order != nil && order.FinalizeURL != "" && (order.Status == acme.StatusReady || order.Status == acme.StatusValid)
}

func validIdentityLeaf(leaf *x509.Certificate, hostname string, now time.Time) bool {
	return leaf.VerifyHostname(hostname) == nil && !now.Before(leaf.NotBefore) && now.Before(leaf.NotAfter)
}

func validCertificateChain(chain [][]byte) bool {
	return len(chain) > 0 && len(chain) <= 10 && certificateChainSize(chain) <= 1<<20
}
