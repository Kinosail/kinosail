package servertest

import (
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"testing"
	"time"

	"github.com/MikeO7/kinosail/packages/identitycore"
	"github.com/MikeO7/kinosail/packages/quickconnect"
)

func TestSecurityFuzzConstructorPreservesAuthorizationLifecycle(t *testing.T) {
	probe := &securityFuzzConstructorProbe{t: t}
	suite := NewSecurityFuzz(nil, nil, probe.initialize, probe.newBroker, probe.bind)
	for _, name := range []string{"first", "second"} {
		profile := identitycore.Profile{ID: name, Name: "Viewer", Remote: true, Libraries: []string{"all"}, Scopes: []string{"library"}}
		previous := probe.broker
		probe.steps = nil
		result := suite.NewAuthorization(profile)
		if !reflect.DeepEqual(profile, *probe.profile) {
			t.Fatal("constructor changed the seeded profile")
		}
		if result.Broker != probe.broker || result.Application != probe.application || result.Broker == previous {
			t.Fatal("constructor changed the bound application or reused prior input state")
		}
		if !slices.Equal(probe.steps, []string{"initialize", "broker", "bind"}) {
			t.Fatalf("authorization construction order = %v", probe.steps)
		}
	}
}

func TestSecurityFuzzConstructorPreservesParserCallbacks(t *testing.T) {
	probe := &securityFuzzConstructorProbe{t: t}
	ip := net.IP{192, 0, 2, 1}
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "https://example.invalid/", nil)
	suite := NewSecurityFuzz(func(value net.IP) bool { return value.Equal(ip) }, func(value *http.Request) bool { return value == request }, probe.initialize, probe.newBroker, probe.bind)
	if !suite.AllowedOutboundIP(ip) || suite.AllowedOutboundIP(nil) {
		t.Fatal("constructor changed the outbound parser input or verdict")
	}
	if !suite.UnsafeCrossOrigin(request) || suite.UnsafeCrossOrigin(nil) {
		t.Fatal("constructor changed the origin parser input or verdict")
	}
	if len(probe.steps) != 0 {
		t.Fatal("parser checks created authorization state")
	}
}

type securityFuzzConstructorProbe struct {
	t           *testing.T
	steps       []string
	profile     *identitycore.Profile
	broker      *quickconnect.Broker
	application *quickconnect.Application
}

func (probe *securityFuzzConstructorProbe) initialize(profile identitycore.Profile, persist func(string, any) error) *identitycore.Profile {
	probe.t.Helper()
	probe.steps = append(probe.steps, "initialize")
	probe.profile = &profile
	path, value := filepath.Join(probe.t.TempDir(), "state.json"), []string{"unchanged"}
	if err := persist(path, value); err != nil {
		probe.t.Fatalf("fuzz persistence policy returned an error: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) || !slices.Equal(value, []string{"unchanged"}) {
		probe.t.Fatal("fuzz persistence policy wrote state or changed its input")
	}
	return probe.profile
}

func (probe *securityFuzzConstructorProbe) newBroker(ttl time.Duration) *quickconnect.Broker {
	probe.t.Helper()
	probe.steps = append(probe.steps, "broker")
	if ttl != time.Minute {
		probe.t.Fatalf("fuzz broker TTL = %v", ttl)
	}
	probe.broker = quickconnect.New(ttl)
	return probe.broker
}

func (probe *securityFuzzConstructorProbe) bind(broker *quickconnect.Broker, profile *identitycore.Profile) RemoteAuthorizationFuzzFixture {
	probe.t.Helper()
	probe.steps = append(probe.steps, "bind")
	if broker != probe.broker || profile != probe.profile {
		probe.t.Fatal("application binding received another broker or profile store")
	}
	probe.application = quickconnect.NewApplication(broker, quickconnect.Limiters{}, quickconnect.Adapter{})
	return RemoteAuthorizationFuzzFixture{Broker: broker, Application: probe.application}
}
