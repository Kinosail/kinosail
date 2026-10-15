package remoteaccess

import (
	"crypto/tls"
	"path/filepath"

	"github.com/MikeO7/kinosail/packages/privatefile"
)

func (manager *Manager) Certificate(name string) *tls.Certificate {
	if manager == nil || !manager.config.PublicHTTPS || name != manager.hostname {
		return nil
	}
	for _, name := range []string{manager.hostname, manager.hostname + "+rsa"} {
		pem, err := privatefile.Read(filepath.Join(manager.config.DataDir, "acme", name), 256<<10)
		if err != nil {
			continue
		}
		cert, err := tls.X509KeyPair(pem, pem)
		if err == nil && manager.validateCertificate(&cert, nil) == nil {
			return &cert
		}
	}
	return nil
}
