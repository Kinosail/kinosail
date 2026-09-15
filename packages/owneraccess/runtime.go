package owneraccess

import (
	"context"
	"crypto/tls"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/MikeO7/kinosail/packages/identitycore"
	"github.com/MikeO7/kinosail/packages/servertransport"
	"golang.zx2c4.com/wireguard/conn"
	"golang.zx2c4.com/wireguard/device"
	"golang.zx2c4.com/wireguard/tun/netstack"
)

// ProfileID is populated only by the private userspace listener, never by an HTTP header.
func ProfileID(r *http.Request) string {
	return identitycore.ManagementProfileID(r)
}

type runtime struct {
	dev         *device.Device
	server      *http.Server
	dns         net.PacketConn
	listener    net.Listener
	mu          sync.Mutex
	connections map[net.Conn]string
	used        map[string]bool
	cancel      context.CancelFunc
}

func (m *Manager) startLocked() error {
	if m.runtime != nil || !validOrigin(m.config.Origin) {
		return errors.New("private management HTTPS is not configured")
	}
	if m.ctx == nil || m.ctx.Err() != nil || m.handler == nil {
		return errors.New("private management is unavailable")
	}
	u, _ := url.Parse(m.config.Origin)
	cert, err := m.config.Certificate(&tls.ClientHelloInfo{ServerName: u.Hostname()})
	if err != nil || cert == nil {
		return errors.New("trusted HTTPS is not ready; finish HTTPS setup first")
	}
	tun, network, err := netstack.CreateNetTUN([]netip.Addr{netip.MustParseAddr(Address)}, nil, 1280)
	if err != nil {
		return errors.New("could not create private management network")
	}
	ctx, cancel := context.WithCancel(m.ctx)
	r := &runtime{dev: device.NewDevice(tun, conn.NewDefaultBind(), device.NewLogger(device.LogLevelSilent, "")), connections: map[net.Conn]string{}, used: map[string]bool{}, cancel: cancel}
	failed := true
	defer func() {
		if failed {
			r.close()
		}
	}()
	if err = r.dev.IpcSet(fmt.Sprintf("private_key=%s\nlisten_port=%d\n", hex.EncodeToString(decodeKey(m.state.PrivateKey)), Port)); err != nil {
		return errors.New("could not configure private management")
	}
	for _, peer := range m.state.Devices {
		r.used[peer.Address] = true
		p, ok := m.config.Profile(peer.ProfileID)
		if ok && eligible(p) && p.Revision == peer.Revision {
			if err = r.add(peer); err != nil {
				return errors.New("could not restore paired devices")
			}
		}
	}
	port := 443
	if u.Port() != "" {
		port, _ = strconv.Atoi(u.Port())
	}
	r.listener, err = network.ListenTCP(&net.TCPAddr{IP: net.ParseIP(Address), Port: port})
	if err != nil {
		return errors.New("could not listen for private management")
	}
	r.dns, err = network.ListenUDP(&net.UDPAddr{IP: net.ParseIP(Address), Port: 53})
	if err != nil {
		return errors.New("could not start private name lookup")
	}
	r.server = servertransport.NewServer("", m.guard(m.handler))
	r.server.TLSConfig.GetCertificate = func(hello *tls.ClientHelloInfo) (*tls.Certificate, error) {
		if hello.ServerName != u.Hostname() {
			return nil, errors.New("management HTTPS name is not allowed")
		}
		return m.config.Certificate(hello)
	}
	r.server.ConnState = r.track
	r.server.BaseContext = func(net.Listener) context.Context { return ctx }
	if err = r.dev.Up(); err != nil {
		return errors.New("could not open management UDP port 51821")
	}
	m.runtime = r
	failed = false
	go func() {
		serveErr := r.server.ServeTLS(r.listener, "", "")
		m.mu.Lock()
		defer m.mu.Unlock()
		if m.runtime == r {
			m.err = serveErr
			m.stopLocked()
		}
	}()
	go m.serveDNS(ctx, r, u.Hostname())
	go m.expirePeers(ctx, r)
	if m.config.MaintainEndpoint != nil {
		go m.maintainEndpoint(ctx, r, m.state.Endpoint)
	}
	return nil
}

func (m *Manager) maintainEndpoint(ctx context.Context, r *runtime, endpoint string) {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for {
		err := m.config.MaintainEndpoint(ctx, endpoint)
		m.mu.Lock()
		if m.runtime != r {
			m.mu.Unlock()
			return
		}
		m.endpointError = err != nil
		m.mu.Unlock()
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (m *Manager) guard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		peer, ok := m.authorizedPeer(r.RemoteAddr)
		if !ok || identitycore.RemoteRequest(r) || !managementHost(r.Host, m.config.Origin) {
			http.Error(w, "private management device is not authorized", http.StatusForbidden)
			return
		}
		// A peer proves a device, not an Owner login. The application must still authenticate it.
		r.Header.Del("X-Kinosail-Proxy-Token")
		r.Header.Del("X-Kinosail-Remote")
		next.ServeHTTP(w, identitycore.WithManagementDevice(r, peer.ProfileID, peer.PublicKey))
	})
}

func managementHost(host, origin string) bool {
	u, _ := url.Parse(origin)
	return host == u.Host || u.Port() == "" && host == u.Host+":443"
}

func (m *Manager) authorizedPeer(address string) (Device, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.state.Enabled || m.runtime == nil {
		return Device{}, false
	}
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return Device{}, false
	}
	for _, peer := range m.state.Devices {
		if peer.Address != host {
			continue
		}
		p, found := m.config.Profile(peer.ProfileID)
		return peer, found && eligible(p) && p.ID == peer.ProfileID && p.Revision == peer.Revision
	}
	return Device{}, false
}

func (m *Manager) expirePeers(ctx context.Context, r *runtime) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.mu.Lock()
			if m.runtime != r {
				m.mu.Unlock()
				return
			}
			if _, err := os.Lstat(filepath.Join(m.config.Directory, "owner-access.disabled")); !errors.Is(err, os.ErrNotExist) {
				m.stopLocked()
				m.state = state{}
				m.retry = false
				m.mu.Unlock()
				return
			}
			for _, peer := range m.state.Devices {
				p, found := m.config.Profile(peer.ProfileID)
				if !found || !eligible(p) || p.Revision != peer.Revision {
					r.remove(peer)
				}
			}
			m.mu.Unlock()
		}
	}
}

func (m *Manager) retryStartup(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.mu.Lock()
			if m.retry && m.state.Enabled && m.runtime == nil {
				if _, err := os.Lstat(filepath.Join(m.config.Directory, "owner-access.disabled")); !errors.Is(err, os.ErrNotExist) {
					m.state, m.retry = state{}, false
				} else {
					m.err = m.startLocked()
				}
			}
			m.mu.Unlock()
		}
	}
}

func (m *Manager) stopLocked() {
	if m.runtime != nil {
		r := m.runtime
		m.runtime = nil
		r.close()
	}
}
func (r *runtime) close() {
	r.cancel()
	if r.server != nil {
		_ = r.server.Close()
	}
	if r.listener != nil {
		_ = r.listener.Close()
	}
	if r.dns != nil {
		_ = r.dns.Close()
	}
	r.dev.Close()
}
func (r *runtime) add(p Device) error {
	return r.dev.IpcSet(fmt.Sprintf("public_key=%s\npreshared_key=%s\nreplace_allowed_ips=true\nallowed_ip=%s/32\n", hex.EncodeToString(decodeKey(p.PublicKey)), hex.EncodeToString(decodeKey(p.PresharedKey)), p.Address))
}
func (r *runtime) remove(p Device) {
	_ = r.dev.IpcSet("public_key=" + hex.EncodeToString(decodeKey(p.PublicKey)) + "\nremove=true\n")
	r.mu.Lock()
	var closing []net.Conn
	for c, address := range r.connections {
		if address == p.Address {
			closing = append(closing, c)
		}
	}
	r.mu.Unlock()
	for _, c := range closing {
		_ = c.Close()
	}
}
func (r *runtime) nextAddress() string {
	for i := 2; i < 255; i++ {
		address := fmt.Sprintf("10.92.0.%d", i)
		if !r.used[address] {
			r.used[address] = true
			return address
		}
	}
	return ""
}
func (r *runtime) track(c net.Conn, state http.ConnState) {
	r.mu.Lock()
	defer r.mu.Unlock()
	switch state {
	case http.StateNew:
		if len(r.connections) >= 64 {
			_ = c.Close()
			return
		}
		host, _, _ := net.SplitHostPort(c.RemoteAddr().String())
		r.connections[c] = host
	case http.StateClosed, http.StateHijacked:
		delete(r.connections, c)
	}
}
