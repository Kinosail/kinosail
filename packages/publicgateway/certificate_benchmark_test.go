package publicgateway

import (
	"crypto/tls"
	"crypto/x509"
	"net"
	"testing"
	"time"
)

func BenchmarkCertificateClientTLSHandshake(b *testing.B) {
	certificate := certificateValidFor(b, 30*24*time.Hour)
	path := certificateSocket(b, CertificateHandler("family.example", func(*tls.ClientHelloInfo) (*tls.Certificate, error) { return &certificate, nil }))
	server := &tls.Config{MinVersion: tls.VersionTLS12, GetCertificate: CertificateClient("family.example", path)}
	leaf, err := x509.ParseCertificate(certificate.Certificate[0])
	if err != nil {
		b.Fatal(err)
	}
	roots := x509.NewCertPool()
	roots.AddCert(leaf)
	client := &tls.Config{MinVersion: tls.VersionTLS13, RootCAs: roots, ServerName: "family.example"}
	b.ReportAllocs()
	for b.Loop() {
		serverConn, clientConn := net.Pipe()
		accepted := make(chan error, 1)
		go func() { accepted <- tls.Server(serverConn, server).HandshakeContext(b.Context()) }()
		if err := tls.Client(clientConn, client).HandshakeContext(b.Context()); err != nil {
			b.Fatal(err)
		}
		if err := <-accepted; err != nil {
			b.Fatal(err)
		}
		_ = serverConn.Close()
		_ = clientConn.Close()
	}
}
