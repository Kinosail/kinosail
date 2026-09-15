package httpguard

import (
	"net/http"
	"net/netip"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	publicQuarantine      = 15 * time.Minute
	trackedSourceLimit    = 4096
	trackedPrefixLimit    = 1024
	trackedPrefixSources  = 32
	globalConcurrentLimit = 64
	sourceConcurrentLimit = 16
	prefixConcurrentLimit = 32
)

type tripwirePrefix struct {
	strikes int
	reset   time.Time
	blocked time.Time
	sources map[string]bool
}

type tripwireSource struct {
	strikes int
	reset   time.Time
	blocked time.Time
}

// PublicTripwire tracks and quarantines abusive public sources.
type PublicTripwire struct {
	mu       sync.Mutex
	sources  map[string]tripwireSource
	prefixes map[string]tripwirePrefix
	overflow tripwireSource
}

// Blocked reports whether a source or its IPv6 prefix is quarantined.
func (tripwire *PublicTripwire) Blocked(key string, now time.Time) bool {
	tripwire.mu.Lock()
	defer tripwire.mu.Unlock()
	state, found := tripwire.sources[key]
	prefix := tripwire.prefixes[abusePrefix(key)]
	return now.Before(state.blocked) || now.Before(prefix.blocked) || !found && len(tripwire.sources) >= trackedSourceLimit && now.Before(tripwire.overflow.blocked)
}

// Strike records abuse and reports whether it caused a quarantine.
func (tripwire *PublicTripwire) Strike(key string, now time.Time, maximum int) bool {
	tripwire.mu.Lock()
	defer tripwire.mu.Unlock()
	if tripwire.sources == nil {
		tripwire.sources = make(map[string]tripwireSource)
	}
	_, found := tripwire.sources[key]
	if !found && len(tripwire.sources) >= trackedSourceLimit {
		for source, state := range tripwire.sources {
			if now.After(state.reset) && now.After(state.blocked) {
				delete(tripwire.sources, source)
			}
		}
		if len(tripwire.sources) >= trackedSourceLimit {
			tripwire.overflow, found = updateTripwireSource(tripwire.overflow, now, maximum)
			return found
		}
	}
	state, blocked := updateTripwireSource(tripwire.sources[key], now, maximum)
	tripwire.sources[key] = state
	return blocked || tripwire.strikePrefix(key, now)
}

func (tripwire *PublicTripwire) strikePrefix(key string, now time.Time) bool { //nolint:cyclop,gocognit // Prefix lifecycle, capacity, and quarantine form one bounded abuse decision.
	prefix := abusePrefix(key)
	if prefix == "" {
		return false
	}
	if tripwire.prefixes == nil {
		tripwire.prefixes = make(map[string]tripwirePrefix)
	}
	state, found := tripwire.prefixes[prefix]
	if !found && len(tripwire.prefixes) >= trackedPrefixLimit {
		for candidate, existing := range tripwire.prefixes {
			if now.After(existing.reset) && now.After(existing.blocked) {
				delete(tripwire.prefixes, candidate)
			}
		}
		if len(tripwire.prefixes) >= trackedPrefixLimit {
			return false
		}
	}
	if now.After(state.reset) {
		state = tripwirePrefix{reset: now.Add(time.Minute), sources: make(map[string]bool)}
	}
	state.strikes++
	if len(state.sources) < trackedPrefixSources {
		state.sources[key] = true
	}
	blocked := state.strikes >= 20 && len(state.sources) >= 4
	if blocked {
		state.blocked = now.Add(publicQuarantine)
	}
	tripwire.prefixes[prefix] = state
	return blocked
}

func abusePrefix(value string) string {
	address, err := netip.ParseAddr(value)
	if err != nil {
		return ""
	}
	address = address.Unmap()
	if address.Is4() {
		return ""
	}
	return netip.PrefixFrom(address, 64).Masked().String()
}

func updateTripwireSource(state tripwireSource, now time.Time, maximum int) (tripwireSource, bool) {
	if now.After(state.reset) {
		state = tripwireSource{reset: now.Add(time.Minute)}
	}
	state.strikes++
	blocked := state.strikes >= maximum
	if blocked {
		state.blocked = now.Add(publicQuarantine)
	}
	return state, blocked
}

// PublicCapacity bounds concurrent public requests by source and IPv6 prefix.
type PublicCapacity struct {
	global   chan struct{}
	mu       sync.Mutex
	sources  map[string]int
	prefixes map[string]int
}

// NewPublicCapacity creates a public request capacity tracker.
func NewPublicCapacity() *PublicCapacity {
	return &PublicCapacity{global: make(chan struct{}, globalConcurrentLimit), sources: make(map[string]int), prefixes: make(map[string]int)}
}

// Acquire reserves capacity for a normalized source key.
func (capacity *PublicCapacity) Acquire(key string) bool {
	select {
	case capacity.global <- struct{}{}:
	default:
		return false
	}
	capacity.mu.Lock()
	defer capacity.mu.Unlock()
	prefix := abusePrefix(key)
	if capacity.sources[key] >= sourceConcurrentLimit || prefix != "" && capacity.prefixes[prefix] >= prefixConcurrentLimit {
		<-capacity.global
		return false
	}
	capacity.sources[key]++
	if prefix != "" {
		capacity.prefixes[prefix]++
	}
	return true
}

// Release returns capacity reserved by a successful Acquire call.
func (capacity *PublicCapacity) Release(key string) {
	capacity.mu.Lock()
	capacity.sources[key]--
	if capacity.sources[key] == 0 {
		delete(capacity.sources, key)
	}
	if prefix := abusePrefix(key); prefix != "" {
		capacity.prefixes[prefix]--
		if capacity.prefixes[prefix] == 0 {
			delete(capacity.prefixes, prefix)
		}
	}
	capacity.mu.Unlock()
	<-capacity.global
}

// SuspiciousPublicPath reports common scanner probes for private files and services.
func SuspiciousPublicPath(path string) bool {
	path = strings.ToLower(path)
	for _, prefix := range []string{"/.env", "/.git", "/.aws", "/.ssh", "/wp-", "/wordpress", "/phpmyadmin", "/cgi-bin", "/server-status", "/actuator", "/vendor/phpunit"} {
		if path == prefix || strings.HasPrefix(path, prefix+"/") || strings.HasPrefix(path, prefix+".") || strings.HasSuffix(prefix, "-") && strings.HasPrefix(path, prefix) {
			return true
		}
	}
	return false
}

// CredentialAttempt reports public routes whose failed POST requests count as authentication abuse.
func CredentialAttempt(request *http.Request) bool {
	if request.Method != http.MethodPost {
		return false
	}
	switch request.URL.Path {
	case "/login", "/login/mfa", "/api/v1/session", "/auth/passkeys/login/finish", "/api/v1/passkeys/login/finish", "/Users/AuthenticateByName", "/Users/AuthenticateWithQuickConnect":
		return true
	}
	return false
}

// ValidPublicRange accepts one unambiguous byte range.
func ValidPublicRange(values []string) bool { //nolint:cyclop // Strict single-range parsing deliberately rejects every ambiguous shape.
	if len(values) == 0 {
		return true
	}
	if len(values) != 1 || strings.Contains(values[0], ",") {
		return false
	}
	unit, interval, found := strings.Cut(strings.TrimSpace(values[0]), "=")
	if !found || !strings.EqualFold(unit, "bytes") || strings.Count(interval, "-") != 1 {
		return false
	}
	start, end, _ := strings.Cut(interval, "-")
	if start == "" && end == "" {
		return false
	}
	var first, last uint64
	var err error
	if start != "" {
		first, err = strconv.ParseUint(start, 10, 64)
		if err != nil {
			return false
		}
	}
	if end != "" {
		last, err = strconv.ParseUint(end, 10, 64)
		if err != nil {
			return false
		}
	}
	return start == "" || end == "" || first <= last
}
