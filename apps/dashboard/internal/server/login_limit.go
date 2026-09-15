package server

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

const (
	loginWindow     = time.Minute
	perSourceLogins = 5
	globalLogins    = 40
	maxLoginSources = 1000
)

type loginBucket struct {
	count int
	until time.Time
}

type loginLimiter struct {
	mu      sync.Mutex
	global  loginBucket
	sources map[string]loginBucket
	now     func() time.Time
}

func newLoginLimiter() *loginLimiter {
	return &loginLimiter{sources: make(map[string]loginBucket), now: time.Now}
}

func (limiter *loginLimiter) allow(request *http.Request, name string) (bool, int) {
	now, key := limiter.now().UTC(), loginSource(request, name)
	limiter.mu.Lock()
	defer limiter.mu.Unlock()
	limiter.global = nextBucket(limiter.global, now)
	if limiter.global.count >= globalLogins {
		return false, max(1, int(limiter.global.until.Sub(now).Seconds()))
	}
	bucket := nextBucket(limiter.sources[key], now)
	if bucket.count >= perSourceLogins {
		limiter.sources[key] = bucket
		return false, max(1, int(bucket.until.Sub(now).Seconds()))
	}
	limiter.global.count++
	bucket.count++
	limiter.sources[key] = bucket
	if len(limiter.sources) > maxLoginSources {
		for source, candidate := range limiter.sources {
			if !candidate.until.After(now) {
				delete(limiter.sources, source)
			}
		}
	}
	return true, 0
}

func (limiter *loginLimiter) success(request *http.Request, name string) {
	limiter.mu.Lock()
	defer limiter.mu.Unlock()
	delete(limiter.sources, loginSource(request, name))
}

func nextBucket(bucket loginBucket, now time.Time) loginBucket {
	if !bucket.until.After(now) {
		return loginBucket{until: now.Add(loginWindow)}
	}
	return bucket
}

func loginSource(request *http.Request, name string) string {
	host, _, err := net.SplitHostPort(request.RemoteAddr)
	if err != nil {
		host = request.RemoteAddr
	}
	name = strings.ToLower(strings.TrimSpace(name))
	if len(name) > 80 {
		name = name[:80]
	}
	return host + "|" + name
}
