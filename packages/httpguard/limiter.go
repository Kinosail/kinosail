package httpguard

import (
	"container/list"
	"log/slog"
	"net"
	"net/netip"
	"strings"
	"sync"
	"time"
)

const trackedClientLimit = 4096

type window struct {
	key   string
	count int
	reset time.Time
}

type Limiter struct {
	mu         sync.Mutex
	windows    map[string]*list.Element
	expiry     list.List
	fullLogged time.Time
}

func (limiter *Limiter) Allow(key string, maximum int) bool {
	key = limiterKey(key)
	limiter.mu.Lock()
	defer limiter.mu.Unlock()
	now := time.Now()
	if limiter.windows == nil {
		limiter.windows = make(map[string]*list.Element)
	}
	element, found := limiter.windows[key]
	if found && now.After(element.Value.(*window).reset) {
		element.Value.(*window).count, element.Value.(*window).reset = 0, now.Add(time.Minute)
		limiter.expiry.MoveToBack(element)
	}
	if !found {
		limiter.dropExpired(now)
		if len(limiter.windows) >= trackedClientLimit {
			limiter.reportFull(now)
			return false
		}
		element = limiter.expiry.PushBack(&window{key: key, reset: now.Add(time.Minute)})
		limiter.windows[key] = element
	}
	current := element.Value.(*window)
	current.count++
	return current.count <= maximum
}

func (limiter *Limiter) dropExpired(now time.Time) {
	for oldest := limiter.expiry.Front(); oldest != nil && now.After(oldest.Value.(*window).reset); oldest = limiter.expiry.Front() {
		delete(limiter.windows, oldest.Value.(*window).key)
		limiter.expiry.Remove(oldest)
	}
}

func (limiter *Limiter) reportFull(now time.Time) {
	if now.Before(limiter.fullLogged) {
		return
	}
	limiter.fullLogged = now.Add(time.Minute)
	slog.Warn("rate limiter refused new clients", "outcome", "table_full", "tracked", len(limiter.windows))
}

func (limiter *Limiter) Reset(key string) {
	key = limiterKey(key)
	limiter.mu.Lock()
	defer limiter.mu.Unlock()
	if element, found := limiter.windows[key]; found {
		delete(limiter.windows, key)
		limiter.expiry.Remove(element)
	}
}

func (limiter *Limiter) TrackedClients() int {
	limiter.mu.Lock()
	defer limiter.mu.Unlock()
	return len(limiter.windows)
}

func limiterKey(key string) string {
	if address, err := netip.ParseAddr(key); err == nil {
		return addressBucket(address)
	}
	if scope, value, found := strings.Cut(key, ":"); found {
		if address, err := netip.ParseAddr(value); err == nil {
			return scope + ":" + addressBucket(address)
		}
	}
	return key
}

func addressBucket(address netip.Addr) string {
	address = address.Unmap().WithZone("")
	if address.Is4() {
		return address.String()
	}
	return netip.PrefixFrom(address, 64).Masked().String()
}

func RemoteHost(remote string) string {
	host, _, err := net.SplitHostPort(remote)
	if err != nil {
		return strings.TrimSpace(remote)
	}
	return host
}
