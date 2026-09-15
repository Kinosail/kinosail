package wireguard

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/MikeO7/kinosail/packages/privatefile"
)

var errInjected = errors.New("injected WireGuard failure")

func TestOpenAndPairReportKeyGenerationFailures(t *testing.T) {
	deps := defaultDependencies()
	deps.keypair = func() (string, string, error) { return "", "", errInjected }
	if _, err := open(t.TempDir(), "host:51820", deps); !errors.Is(err, errInjected) {
		t.Fatalf("open() error = %v", err)
	}
	manager, err := Open(t.TempDir(), "host:51820")
	if err != nil {
		t.Fatal(err)
	}
	manager.deps.keypair = deps.keypair
	if _, err = manager.Pair("Viewer", "viewer"); !errors.Is(err, errInjected) {
		t.Fatalf("Pair() keypair error = %v", err)
	}
	manager.deps.keypair = defaultDependencies().keypair
	manager.deps.randomKey = func() (string, error) { return "", errInjected }
	if _, err = manager.Pair("Viewer", "viewer"); !errors.Is(err, errInjected) {
		t.Fatalf("Pair() preshared key error = %v", err)
	}
}

func TestWireGuardCoversAddressExhaustionAndMissingValidKey(t *testing.T) {
	manager, err := Open(t.TempDir(), "host:51820")
	if err != nil {
		t.Fatal(err)
	}
	for octet := 2; octet < 255; octet++ {
		manager.state.Peers = append(manager.state.Peers, Viewer{Address: fmt.Sprintf("10.91.0.%d", octet)})
	}
	if address := manager.nextAddress(); address != "" {
		t.Fatalf("exhausted address = %q", address)
	}
	manager.state.Peers = nil
	_, missingKey, err := manager.deps.keypair()
	if err != nil {
		t.Fatal(err)
	}
	if err := manager.Revoke(missingKey); err == nil {
		t.Fatal("missing valid key was revoked")
	}
}

func TestKeypairReportsRandomSourceFailure(t *testing.T) {
	if private, public, err := keypair(failingReader{}); err == nil || private != "" || public != "" {
		t.Fatalf("keypair() = %q, %q, %v", private, public, err)
	}
	if key, err := randomKey(failingReader{}); err == nil || key != "" {
		t.Fatalf("randomKey() = %q, %v", key, err)
	}
}

func TestSaveReportsEachDependencyFailure(t *testing.T) {
	for name, fail := range map[string]func(*dependencies){
		"mkdir":           func(deps *dependencies) { deps.mkdirAll = func(string, os.FileMode) error { return errInjected } },
		"directory chmod": func(deps *dependencies) { deps.chmod = failCall(1) },
		"config chmod":    func(deps *dependencies) { deps.chmod = failCall(2) },
		"marshal": func(deps *dependencies) {
			deps.marshal = func(state) ([]byte, error) { return nil, errInjected }
		},
		"config write": func(deps *dependencies) { deps.write = failWrite(1) },
		"state write":  func(deps *dependencies) { deps.write = failWrite(2) },
	} {
		t.Run(name, func(t *testing.T) {
			manager := &Manager{directory: filepath.Join(t.TempDir(), "wireguard"), deps: defaultDependencies()}
			fail(&manager.deps)
			if err := manager.save(); !errors.Is(err, errInjected) {
				t.Fatalf("save() error = %v", err)
			}
		})
	}
}

type failingReader struct{}

func (failingReader) Read([]byte) (int, error) { return 0, errInjected }

func failCall(target int) func(string, os.FileMode) error {
	calls := 0
	return func(path string, mode os.FileMode) error {
		calls++
		if calls == target {
			return errInjected
		}
		return os.Chmod(path, mode)
	}
}

func failWrite(target int) func(string, []byte) error {
	calls := 0
	return func(path string, data []byte) error {
		calls++
		if calls == target {
			return errInjected
		}
		return privatefile.Write(path, data)
	}
}
