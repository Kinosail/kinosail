package httpguard

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"
)

func TestPublicTripwireBoundsTrackedSources(t *testing.T) {
	t.Parallel()
	var tripwire PublicTripwire
	now := time.Now()
	for source := 0; source < trackedSourceLimit+1; source++ {
		tripwire.Strike("source-"+strconv.Itoa(source), now, 3)
	}
	if len(tripwire.sources) > trackedSourceLimit {
		t.Fatalf("tripwire tracked %d sources", len(tripwire.sources))
	}
	tripwire.Strike("overflow-1", now, 3)
	tripwire.Strike("overflow-2", now, 3)
	if !tripwire.Blocked("new-source", now) || len(tripwire.sources) != trackedSourceLimit {
		t.Fatalf("saturated tripwire did not fail closed: sources=%d", len(tripwire.sources))
	}
}

func TestPublicTripwireCompoundsTemporaryIPv6AddressesWithoutPrefixLockoutFromOneSource(t *testing.T) {
	t.Parallel()
	var tripwire PublicTripwire
	now := time.Now()
	for range 20 {
		tripwire.Strike("2001:db8:1:2::1", now, 100)
	}
	if tripwire.Blocked("2001:db8:1:2::99", now) {
		t.Fatal("one IPv6 source quarantined its whole prefix")
	}
	for attempt := 1; attempt <= 20; attempt++ {
		tripwire.Strike("2001:db8:1:2::"+strconv.Itoa(attempt), now, 100)
	}
	if !tripwire.Blocked("2001:db8:1:2::ffff", now) || tripwire.Blocked("2001:db8:1:3::1", now) {
		t.Fatal("distributed IPv6 abuse was not isolated to its /64")
	}
	if abusePrefix("::ffff:192.0.2.1") != "" || abusePrefix("malformed") != "" {
		t.Fatal("IPv4-mapped or malformed source received an IPv6 prefix bucket")
	}
}

func TestPublicCapacityBoundsSourcesAndIPv6Prefixes(t *testing.T) {
	t.Parallel()
	capacity := NewPublicCapacity()
	keys := make([]string, prefixConcurrentLimit)
	for index := range keys {
		keys[index] = "2001:db8:1:2::" + strconv.Itoa(index+1)
		if !capacity.Acquire(keys[index]) {
			t.Fatalf("prefix request %d rejected too early", index)
		}
	}
	if capacity.Acquire("2001:db8:1:2::ffff") {
		t.Fatal("rotating IPv6 address bypassed the /64 concurrency bound")
	}
	if !capacity.Acquire("2001:db8:1:3::1") {
		t.Fatal("one busy /64 blocked an unrelated prefix")
	}
	capacity.Release("2001:db8:1:3::1")
	for _, key := range keys {
		capacity.Release(key)
	}
	for range sourceConcurrentLimit {
		if !capacity.Acquire("192.0.2.1") {
			t.Fatal("source request was rejected before its limit")
		}
	}
	if capacity.Acquire("192.0.2.1") {
		t.Fatal("source concurrency limit was bypassed")
	}
	for range sourceConcurrentLimit {
		capacity.Release("192.0.2.1")
	}
}

func TestPublicRequestClassifiersRejectAmbiguousInputs(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		values []string
		want   bool
	}{
		{nil, true},
		{[]string{"bytes=0-1"}, true},
		{[]string{"bytes=-500"}, true},
		{[]string{"bytes=1-"}, true},
		{[]string{"bytes="}, false},
		{[]string{"bytes=1-0"}, false},
		{[]string{"bytes=0-1,2-3"}, false},
		{[]string{"bytes=0-1", "bytes=2-3"}, false},
		{[]string{"items=0-1"}, false},
		{[]string{"bytes=18446744073709551616-"}, false},
	} {
		if got := ValidPublicRange(test.values); got != test.want {
			t.Errorf("ValidPublicRange(%q) = %t; want %t", test.values, got, test.want)
		}
	}
	for path, want := range map[string]bool{
		"/.env":         true,
		"/.GIT/config":  true,
		"/wp-login.php": true,
		"/environment":  false,
		"/login":        false,
	} {
		if got := SuspiciousPublicPath(path); got != want {
			t.Errorf("SuspiciousPublicPath(%q) = %t; want %t", path, got, want)
		}
	}
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/v1/session", nil)
	if !CredentialAttempt(request) {
		t.Fatal("credential POST was not classified")
	}
	request.Method = http.MethodGet
	if CredentialAttempt(request) {
		t.Fatal("credential GET was classified as an attempt")
	}
}

func TestPublicGuardRemainingCapacityAndExpiryEdges(t *testing.T) { //nolint:cyclop,gocognit // One bounded-state matrix covers independent capacity cleanup paths.
	t.Parallel()
	now := time.Now()
	tripwire := PublicTripwire{sources: make(map[string]tripwireSource, trackedSourceLimit)}
	for index := range trackedSourceLimit {
		tripwire.sources["expired-"+strconv.Itoa(index)] = tripwireSource{reset: now.Add(-time.Minute), blocked: now.Add(-time.Minute)}
	}
	if tripwire.Strike("replacement", now, 3) || len(tripwire.sources) != 1 {
		t.Fatalf("expired sources were not reclaimed: %d", len(tripwire.sources))
	}

	prefixes := make(map[string]tripwirePrefix, trackedPrefixLimit)
	for index := range trackedPrefixLimit {
		prefixes["2001:db8:"+strconv.Itoa(index)+"::/64"] = tripwirePrefix{reset: now.Add(time.Minute)}
	}
	tripwire = PublicTripwire{prefixes: prefixes}
	if tripwire.strikePrefix("2001:db8:ffff::1", now) || len(tripwire.prefixes) != trackedPrefixLimit {
		t.Fatal("saturated prefix tracking did not fail closed")
	}
	for key := range tripwire.prefixes {
		tripwire.prefixes[key] = tripwirePrefix{reset: now.Add(-time.Minute), blocked: now.Add(-time.Minute)}
		break
	}
	if tripwire.strikePrefix("2001:db8:ffff::1", now) {
		t.Fatal("new prefix was quarantined on its first strike")
	}

	capacity := NewPublicCapacity()
	for index := range globalConcurrentLimit {
		if !capacity.Acquire("192.0.2." + strconv.Itoa(index+1)) {
			t.Fatalf("global request %d rejected early", index)
		}
	}
	if capacity.Acquire("198.51.100.1") {
		t.Fatal("global concurrency limit was bypassed")
	}
	for index := range globalConcurrentLimit {
		capacity.Release("192.0.2." + strconv.Itoa(index+1))
	}

	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/public/content", nil)
	if CredentialAttempt(request) {
		t.Fatal("ordinary public POST was classified as a credential attempt")
	}
	for _, value := range []string{"bytes=-", "bytes=0-18446744073709551616"} {
		if ValidPublicRange([]string{value}) {
			t.Fatalf("invalid range %q was accepted", value)
		}
	}
}
