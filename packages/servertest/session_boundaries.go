package servertest

import (
	"net"
	"testing"
	"time"

	"github.com/MikeO7/kinosail/packages/identitycore"
)

// PublicOutboundResolutionRejectsMixedAndEmptyAnswers verifies fail-closed DNS resolution.
func PublicOutboundResolutionRejectsMixedAndEmptyAnswers(t *testing.T, allowed func(net.IP) bool) {
	t.Parallel()
	for name, addresses := range map[string][]net.IP{
		"mixed public and loopback": {net.ParseIP("203.0.113.10"), net.ParseIP("127.0.0.1")},
		"mixed public and private":  {net.ParseIP("203.0.113.10"), net.ParseIP("10.0.0.2")},
		"empty":                     {},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := identitycore.AllowedResolvedAddress(addresses, allowed); err == nil {
				t.Fatal("unsafe DNS answer was accepted")
			}
		})
	}
	address, err := identitycore.AllowedResolvedAddress([]net.IP{net.ParseIP("203.0.113.10"), net.ParseIP("2001:db8::1")}, allowed)
	if err != nil || !address.Equal(net.ParseIP("203.0.113.10")) {
		t.Fatalf("public-only answer = %v, %v", address, err)
	}
}

// SessionTimeoutValidationHasNoSideEffects verifies all invalid boundary combinations.
func SessionTimeoutValidationHasNoSideEffects(t *testing.T, read func() (time.Duration, time.Duration), set func(float64, float64) error) {
	beforeInactive, beforeAbsolute := read()
	for _, values := range [][2]float64{{0, 4}, {.24, 4}, {1, 3}, {8761, 8761}, {24, 8761}, {48, 24}} {
		if err := set(values[0], values[1]); err == nil {
			t.Fatalf("accepted timeouts %v", values)
		}
	}
	afterInactive, afterAbsolute := read()
	if beforeInactive != afterInactive || beforeAbsolute != afterAbsolute {
		t.Fatalf("invalid input changed timeouts from %s/%s to %s/%s", beforeInactive, beforeAbsolute, afterInactive, afterAbsolute)
	}
}
