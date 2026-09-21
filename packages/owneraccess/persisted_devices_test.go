package owneraccess

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail/packages/privatefile"
)

func TestManagementReloadValidatesEveryPersistedPeerBeforeUse(t *testing.T) {
	config, _, _ := ownerFixture(t)
	key := base64.StdEncoding.EncodeToString([]byte(strings.Repeat("a", 32)))
	peer := Device{Label: "Phone", ProfileID: "owner", Revision: 1, PublicKey: key, PresharedKey: key, Address: "10.92.0.2"}
	valid := state{Enabled: true, Endpoint: "home.example:51821", PrivateKey: key, Devices: []Device{peer}}
	path := filepath.Join(config.Directory, "owner-access.json")
	raw, err := json.Marshal(valid) // #nosec G117 -- Synthetic keys are persisted to a private test directory.
	if err != nil {
		t.Fatal(err)
	}
	if err := privatefile.Write(path, raw); err != nil {
		t.Fatal(err)
	}
	manager, err := Open(config)
	if err != nil || len(manager.Status().Devices) != 1 {
		t.Fatalf("valid persisted peer rejected: %v", err)
	}
	for _, mutate := range []func(*state){
		func(s *state) { s.Devices = make([]Device, maxDevices+1) },
		func(s *state) { s.Devices = append(s.Devices, s.Devices[0]) },
		func(s *state) {
			other := s.Devices[0]
			other.PublicKey = base64.StdEncoding.EncodeToString([]byte(strings.Repeat("b", 32)))
			s.Devices = append(s.Devices, other)
		},
		func(s *state) { s.Devices[0].Label = "bad\nlabel" },
		func(s *state) { s.Devices[0].ProfileID = "bad owner" },
		func(s *state) { s.Devices[0].Revision = 0 },
		func(s *state) { s.Devices[0].PublicKey = "bad" },
		func(s *state) { s.Devices[0].PresharedKey = "bad" },
		func(s *state) { s.Devices[0].Address = "10.92.0.1" },
		func(s *state) { s.Devices[0].Address = "10.92.0.255" },
		func(s *state) { s.Devices[0].Address = "192.168.1.2" },
		func(s *state) { s.Devices[0].Address = "not-an-address" },
	} {
		candidate := valid
		candidate.Devices = []Device{peer}
		mutate(&candidate)
		assertInvalidPersistedState(t, config, path, candidate)
	}
}

func assertInvalidPersistedState(t *testing.T, config Config, path string, candidate state) {
	t.Helper()
	data, err := json.Marshal(candidate) // #nosec G117 -- Synthetic invalid state exercises private-file loading.
	if err != nil {
		t.Fatal(err)
	}
	if err := privatefile.Write(path, data); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(config); err == nil {
		t.Fatal("invalid persisted peer accepted")
	}
	after, err := os.ReadFile(path)
	if err != nil || string(after) != string(data) {
		t.Fatal("rejected state was modified")
	}
}
