package remoteaccess_test

import (
	"crypto/tls"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail/packages/remoteaccess"
)

func TestResetKillIsStrictAndRemovesOnlyTheKillMarker(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()
	config := remoteaccess.Config{Enabled: true, PublicHTTPS: true, Domain: "family", Token: strings.Repeat("k", 32), Listen: "127.0.0.1:8443", DataDir: directory}
	manager, err := remoteaccess.New(config, remoteaccess.Dependencies{Certificate: func(*tls.ClientHelloInfo) (*tls.Certificate, error) { return nil, nil }})
	if err != nil || manager.Kill() != nil || manager.ResetKill() != nil {
		t.Fatalf("kill/reset = manager:%#v error:%v", manager, err)
	}
	if manager.Status().State != "starting" {
		t.Fatalf("reset status = %#v", manager.Status())
	}
	if _, err = os.Stat(filepath.Join(directory, "remote-access.disabled")); !os.IsNotExist(err) {
		t.Fatalf("kill marker remains: %v", err)
	}
	disabled, err := remoteaccess.New(remoteaccess.Config{})
	if err != nil || disabled.ResetKill() == nil {
		t.Fatalf("disabled reset = %#v, %v", disabled, err)
	}
}
