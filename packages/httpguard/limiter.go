package httpguard

import (
	"net"
	"strings"
	"sync"
	"time"
)

const trackedClientLimit = 1024

type window struct {
	count int
	reset time.Time
}

// Limiter bounds one-minute request counters and tracked client state.
type Limiter struct {
	mu      sync.Mutex
	windows map[string]window
}

// Allow records a request and reports whether it is inside the supplied limit.
func (limiter *Limiter) Allow(key string, maximum int) bool {
	now := time.Now()
	limiter.mu.Lock()
	defer limiter.mu.Unlock()
	if limiter.windows == nil {
		limiter.windows = make(map[string]window)
	}
	current, found := limiter.windows[key]
	if !found && len(limiter.windows) >= trackedClientLimit {
		for candidate, tracked := range limiter.windows {
			if now.After(tracked.reset) {
				delete(limiter.windows, candidate)
			}
		}
		if len(limiter.windows) >= trackedClientLimit {
			return false
		}
	}
	if now.After(current.reset) {
		current = window{reset: now.Add(time.Minute)}
	}
	current.count++
	limiter.windows[key] = current
	return current.count <= maximum
}

// Reset removes the request counter for one normalized key.
func (limiter *Limiter) Reset(key string) {
	limiter.mu.Lock()
	delete(limiter.windows, key)
	limiter.mu.Unlock()
}

// TrackedClients reports the number of active client counters.
func (limiter *Limiter) TrackedClients() int {
	limiter.mu.Lock()
	defer limiter.mu.Unlock()
	return len(limiter.windows)
}

// RemoteHost removes a valid port from a remote network address.
func RemoteHost(remote string) string {
	host, _, err := net.SplitHostPort(remote)
	if err != nil {
		return strings.TrimSpace(remote)
	}
	return host
}
