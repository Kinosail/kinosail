package owneraccess

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/MikeO7/kinosail/packages/identitycore"
	"github.com/MikeO7/kinosail/packages/privatefile"
	"golang.zx2c4.com/wireguard/conn"
	"golang.zx2c4.com/wireguard/device"
	"golang.zx2c4.com/wireguard/tun/netstack"
)

func ownerFixture(t *testing.T) (Config, *identitycore.Profile, *sync.Mutex) {
	t.Helper()
	profile := &identitycore.Profile{ID: "owner", Owner: true, TOTPSecret: "enrolled", Revision: 1}
	lock := &sync.Mutex{}
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	template := &x509.Certificate{SerialNumber: big.NewInt(1), DNSNames: []string{"server.example"}, NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(24 * time.Hour), KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	cert := &tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key}
	config := Config{Directory: t.TempDir(), Origin: "https://server.example", Profile: func(id string) (identitycore.Profile, bool) {
		lock.Lock()
		defer lock.Unlock()
		return *profile, id == profile.ID
	}, Certificate: func(*tls.ClientHelloInfo) (*tls.Certificate, error) { return cert, nil }}
	return config, profile, lock
}

func TestManagementDefaultsOffAndInvalidInputHasNoEffects(t *testing.T) {
	config, _, _ := ownerFixture(t)
	m, err := Open(config)
	if err != nil {
		t.Fatal(err)
	}
	if status := m.Status(); status.Enabled || status.State != "off" || len(status.Devices) != 0 {
		t.Fatalf("default %+v", status)
	}
	for _, endpoint := range []string{"", "server.example", "http://server.example:51821", "server.example:0", "server.example:051821", "server.example:65536", "server.example:53", "127.0.0.1:51821", "server.example:51821\nprivate_key=x", strings.Repeat("x", 254) + ":51821"} {
		if m.Enable(endpoint) == nil {
			t.Fatalf("accepted endpoint %q", endpoint)
		}
	}
	for _, input := range []struct{ label, id string }{{"", "owner"}, {"phone\nprivate_key=x", "owner"}, {strings.Repeat("p", 81), "owner"}, {"phone", ""}, {"phone", "missing"}, {"phone", "owner owner"}} {
		if _, err = m.Pair(input.label, input.id); err == nil {
			t.Fatalf("accepted pairing %+v", input)
		}
	}
	if m.Revoke("invalid") == nil {
		t.Fatal("accepted invalid revocation")
	}
	entries, err := os.ReadDir(config.Directory)
	if err != nil || len(entries) != 0 {
		t.Fatalf("rejected input wrote state: %v %v", entries, err)
	}
}

func TestManagementRejectsInvalidPersistedStateWithoutChangingIt(t *testing.T) {
	config, _, _ := ownerFixture(t)
	for _, raw := range []string{`null`, `{"enabled":true}`, `{"enabled":false,"enabled":true}`, `{"enabled":false,"unknown":"x"}`, strings.Repeat(" ", 64<<10+1)} {
		path := filepath.Join(config.Directory, "owner-access.json")
		if err := privatefile.Write(path, []byte(raw)); err != nil {
			t.Fatal(err)
		}
		if _, err := Open(config); err == nil {
			t.Fatalf("accepted state %q", raw[:min(100, len(raw))])
		}
		data, err := os.ReadFile(path)
		if err != nil || string(data) != raw {
			t.Fatal("invalid state was changed")
		}
	}
	if err := Recover(config.Directory); err != nil {
		t.Fatal(err)
	}
	m, err := Open(config)
	if err != nil || m.Status().Enabled {
		t.Fatalf("local recovery = %v", err)
	}
}

func TestManagementTunnelBindsPeerRevokesLiveStreamAndPreservesPrivateBoundary(t *testing.T) {
	// One real UDP listener, intentionally not parallel with another management runtime.
	config, p, lock := ownerFixture(t)
	m, err := Open(config)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	m.Attach(ctx, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if ProfileID(r) != "owner" || r.Header.Get("X-Kinosail-Proxy-Token") != "" {
			t.Error("device binding missing")
		}
		if r.URL.Path == "/stream" {
			w.WriteHeader(200)
			_, _ = w.Write([]byte("ready"))
			w.(http.Flusher).Flush()
			<-r.Context().Done()
			return
		}
		_, _ = w.Write([]byte(ProfileID(r)))
	}))
	if err = m.Enable("home.example:51821"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = m.Disable() })
	for _, change := range []func(*identitycore.Profile){func(p *identitycore.Profile) { p.Owner = false }, func(p *identitycore.Profile) { p.Disabled = true }, func(p *identitycore.Profile) { p.SCIMDeleted = true }, func(p *identitycore.Profile) { p.TOTPSecret = "" }} {
		lock.Lock()
		original := *p
		change(p)
		lock.Unlock()
		if _, err = m.Pair("phone", "owner"); err == nil {
			t.Fatal("ineligible profile paired")
		}
		lock.Lock()
		*p = original
		lock.Unlock()
	}
	profile, err := m.Pair("Phone", "owner")
	if err != nil {
		t.Fatal(err)
	}
	status := m.Status()
	if len(status.Devices) != 1 || status.Devices[0].PresharedKey != "" || strings.Contains(fmt.Sprint(status), "PrivateKey") {
		t.Fatal("status exposed keys")
	}
	client, network := tunnelClient(t, config, profile)
	response, err := client.Get(config.Origin + "/settings")
	if err != nil {
		t.Fatal(err)
	}
	body, err := io.ReadAll(response.Body)
	_ = response.Body.Close()
	if err != nil || string(body) != "owner" {
		t.Fatalf("management = %q %v", body, err)
	}
	// Altering an HTTP header on the LAN never produces a paired-device context.
	r := httptest.NewRequest("GET", config.Origin, nil)
	r.Header.Set("X-Kinosail-Owner-Device", "owner")
	if ProfileID(r) != "" {
		t.Fatal("header established identity")
	}
	for _, address := range []string{"10.92.0.1:22", "10.92.0.3:443", "192.168.1.1:80"} {
		dialCtx, stop := context.WithTimeout(ctx, 100*time.Millisecond)
		c, err := network.DialContext(dialCtx, "tcp", address)
		stop()
		if err == nil {
			_ = c.Close()
			t.Fatalf("tunnel reached %s", address)
		}
	}
	response, err = client.Get(config.Origin + "/stream")
	if err != nil {
		t.Fatal(err)
	}
	start := make([]byte, 5)
	if _, err = io.ReadFull(response.Body, start); err != nil {
		t.Fatal(err)
	}
	if err = m.Revoke(status.Devices[0].PublicKey); err != nil {
		t.Fatal(err)
	}
	if _, err = response.Body.Read(make([]byte, 1)); err == nil {
		t.Fatal("revoked stream stayed open")
	}
	_ = response.Body.Close()
	if len(m.Status().Devices) != 0 {
		t.Fatal("revoked device remained")
	}
	newProfile, err := m.Pair("Replacement", "owner")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(newProfile, "Address = "+status.Devices[0].Address+"/32") {
		t.Fatal("revoked source address reused in active runtime")
	}
	lock.Lock()
	p.Revision++
	lock.Unlock()
	if _, allowed := m.authorizedPeer(m.Status().Devices[0].Address + ":1234"); allowed {
		t.Fatal("credential change retained device access")
	}
	if err = m.Disable(); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(config)
	if err != nil || reopened.Status().Enabled || len(reopened.Status().Devices) != 0 {
		t.Fatalf("restart restored disabled peers: %v", err)
	}
}

func tunnelClient(t *testing.T, config Config, profile string) (*http.Client, *netstack.Net) {
	t.Helper()
	values := map[string]string{}
	for _, line := range strings.Split(profile, "\n") {
		key, value, ok := strings.Cut(line, " = ")
		if ok {
			values[key] = value
		}
	}
	address := strings.TrimSuffix(values["Address"], "/32")
	tun, network, err := netstack.CreateNetTUN([]netip.Addr{netip.MustParseAddr(address)}, nil, 1280)
	if err != nil {
		t.Fatal(err)
	}
	dev := device.NewDevice(tun, conn.NewDefaultBind(), device.NewLogger(device.LogLevelSilent, ""))
	t.Cleanup(dev.Close)
	ipc := fmt.Sprintf("private_key=%s\npublic_key=%s\npreshared_key=%s\nendpoint=127.0.0.1:51821\nallowed_ip=10.92.0.1/32\n", hex.EncodeToString(decodeKey(values["PrivateKey"])), hex.EncodeToString(decodeKey(values["PublicKey"])), hex.EncodeToString(decodeKey(values["PresharedKey"])))
	if err = dev.IpcSet(ipc); err != nil {
		t.Fatal(err)
	}
	if err = dev.Up(); err != nil {
		t.Fatal(err)
	}
	cert, err := config.Certificate(&tls.ClientHelloInfo{ServerName: "server.example"})
	if err != nil {
		t.Fatal(err)
	}
	leaf, err := x509.ParseCertificate(cert.Certificate[0])
	if err != nil {
		t.Fatal(err)
	}
	roots := x509.NewCertPool()
	roots.AddCert(leaf)
	transport := &http.Transport{TLSClientConfig: &tls.Config{RootCAs: roots, ServerName: "server.example", MinVersion: tls.VersionTLS12}, DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
		return network.DialContext(ctx, "tcp", "10.92.0.1:443")
	}}
	t.Cleanup(transport.CloseIdleConnections)
	return &http.Client{Transport: transport, Timeout: 5 * time.Second}, network
}
