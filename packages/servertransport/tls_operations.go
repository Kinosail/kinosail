package servertransport

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"io"
	"math/big"
	"os"

	"github.com/MikeO7/kinosail/packages/privatefile"
)

type tlsOperations struct {
	mkdirAll          func(string, os.FileMode) error
	generateKey       func() (*ecdsa.PrivateKey, error)
	serialNumber      func() (*big.Int, error)
	createCertificate func(*x509.Certificate, *x509.Certificate, *ecdsa.PublicKey, *ecdsa.PrivateKey) ([]byte, error)
	marshalPrivateKey func(*ecdsa.PrivateKey) ([]byte, error)
	encodePEM         func(io.Writer, *pem.Block) error
	readPrivate       func(string, int64) ([]byte, error)
	writePrivate      func(string, []byte) error
}

func defaultTLSOperations() tlsOperations {
	return tlsOperations{
		mkdirAll:     os.MkdirAll,
		generateKey:  func() (*ecdsa.PrivateKey, error) { return ecdsa.GenerateKey(elliptic.P256(), rand.Reader) },
		serialNumber: serialNumber,
		createCertificate: func(template, parent *x509.Certificate, public *ecdsa.PublicKey, private *ecdsa.PrivateKey) ([]byte, error) {
			return x509.CreateCertificate(rand.Reader, template, parent, public, private)
		},
		marshalPrivateKey: func(key *ecdsa.PrivateKey) ([]byte, error) { return x509.MarshalPKCS8PrivateKey(key) },
		encodePEM:         pem.Encode,
		readPrivate:       privatefile.Read,
		writePrivate:      privatefile.Write,
	}
}
