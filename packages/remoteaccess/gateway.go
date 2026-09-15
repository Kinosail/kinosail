package remoteaccess

import (
	"context"
	"errors"
	"net"
	"net/http"
	"os"
	"path/filepath"

	"github.com/MikeO7/kinosail/packages/identitycore"
	"github.com/MikeO7/kinosail/packages/publicgateway"
	"github.com/MikeO7/kinosail/packages/servertransport"
)

// serveGateway never binds an IP listener in the process holding Owner state.
// Socket ownership and the gateway's read-only mount provide the transport boundary;
// the application independently enforces Remote on every request, including forged headers.
func (manager *Manager) serveGateway(ctx context.Context, handler http.Handler) error {
	listener, err := privateListener(publicgateway.HTTPPath)
	if err != nil {
		return err
	}
	defer listener.Close()
	certListener, err := privateListener(publicgateway.CertificatePath)
	if err != nil {
		return err
	}
	defer certListener.Close()
	server := servertransport.NewServer("", publicgateway.TLSMetadata(identitycore.Remote(exactPublicHost(manager.hostname, handler))))
	server.ConnState = manager.trackConnection
	certServer := servertransport.NewServer("", publicgateway.CertificateHandler(manager.hostname, manager.certificate))
	certServer.ConnState = manager.trackConnection
	defer certServer.Close()
	manager.mu.Lock()
	if manager.killed {
		manager.mu.Unlock()
		return nil
	}
	manager.server, manager.listener = server, listener
	manager.status.Isolation = "public-gateway"
	manager.mu.Unlock()
	defer func() { manager.mu.Lock(); manager.server, manager.listener = nil, nil; manager.mu.Unlock() }()
	serveCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	go func() { <-serveCtx.Done(); _ = server.Close(); _ = certServer.Close() }()
	go func() { _ = certServer.Serve(certListener); _ = server.Close() }()
	go manager.refresh(serveCtx)
	manager.setStatus("ready", nil)
	err = server.Serve(listener)
	if errors.Is(err, http.ErrServerClosed) || errors.Is(err, net.ErrClosed) {
		return nil
	}
	manager.setStatus("error", err)
	return err
}

func privateListener(path string) (net.Listener, error) {
	if filepath.Dir(path) != publicgateway.Directory {
		return nil, errors.New("invalid public socket directory")
	}
	if err := os.MkdirAll(publicgateway.Directory, 0o700); err != nil {
		return nil, err
	}
	if err := os.Chmod(publicgateway.Directory, 0o700); err != nil {
		return nil, err
	}
	if info, err := os.Lstat(path); err == nil {
		if info.Mode()&os.ModeSocket == 0 {
			return nil, errors.New("public socket path is occupied")
		}
		if err = os.Remove(path); err != nil {
			return nil, err
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	l, err := net.Listen("unix", path)
	if err != nil {
		return nil, err
	}
	if err = os.Chmod(path, 0o600); err != nil {
		_ = l.Close()
		return nil, err
	}
	return l, nil
}
