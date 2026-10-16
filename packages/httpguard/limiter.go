package httpguard

import (
	"container/list"
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
	mu      sync.Mutex
	windows map[string]*list.Element
	recent  list.List
}

func (limiter *Limiter) Allow(key string, maximum int) bool {
	now := time.Now()
	key = limiterKey(key)
	limiter.mu.Lock()
	defer limiter.mu.Unlock()
	if limiter.windows == nil {
		limiter.windows = make(map[string]*list.Element)
	}
	element, found := limiter.windows[key]
	if !found {
		if len(limiter.windows) >= trackedClientLimit {
			oldest := limiter.recent.Back()
			delete(limiter.windows, oldest.Value.(*window).key)
			limiter.recent.Remove(oldest)
		}
		element = limiter.recent.PushFront(&window{key: key})
		limiter.windows[key] = element
	}
	limiter.recent.MoveToFront(element)
	current := element.Value.(*window)
	if now.After(current.reset) {
		current.count, current.reset = 0, now.Add(time.Minute)
	}
	current.count++
	return current.count <= maximum
}

func (limiter *Limiter) Reset(key string) {
	key = limiterKey(key)
	limiter.mu.Lock()
	defer limiter.mu.Unlock()
	if element, found := limiter.windows[key]; found {
		delete(limiter.windows, key)
		limiter.recent.Remove(element)
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
