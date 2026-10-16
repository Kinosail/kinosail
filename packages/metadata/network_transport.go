package metadata

import (
	"context"
	"io"
	"net/http"
	"strconv"
	"sync"
	"time"
)

const (
	providerIdleConnections   = 16
	providerRequestsPerSecond = 40
	providerBurst             = 10
	providerRetries           = 2
	maximumRetryAfter         = 30 * time.Second
	providerDrainLimit        = 64 << 10
)

type providerTransport struct {
	next    http.RoundTripper
	mu      sync.Mutex
	buckets map[string]*providerBucket
}

type providerBucket struct {
	tokens   float64
	updated  time.Time
	resumeAt time.Time
}

type drainingBody struct{ io.ReadCloser }

func (body drainingBody) Close() error {
	_, _ = io.CopyN(io.Discard, body.ReadCloser, providerDrainLimit)
	return body.ReadCloser.Close()
}

func newProviderTransport(next http.RoundTripper) *providerTransport {
	return &providerTransport{next: next, buckets: make(map[string]*providerBucket)}
}

func (transport *providerTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	for attempt := 0; ; attempt++ {
		if err := transport.wait(request.Context(), request.URL.Host); err != nil {
			return nil, err
		}
		response, err := transport.next.RoundTrip(request)
		if err != nil {
			return nil, err
		}
		delay, retry := retryDelay(request, response, attempt)
		if !retry {
			response.Body = drainingBody{response.Body}
			return response, nil
		}
		_ = drainingBody{response.Body}.Close()
		transport.pause(request.URL.Host, delay)
	}
}

func retryDelay(request *http.Request, response *http.Response, attempt int) (time.Duration, bool) {
	if attempt >= providerRetries || request.Body != nil && request.Body != http.NoBody || request.Method != http.MethodGet && request.Method != http.MethodHead {
		return 0, false
	}
	if response.StatusCode != http.StatusTooManyRequests && response.StatusCode != http.StatusServiceUnavailable {
		return 0, false
	}
	header := response.Header.Get("Retry-After")
	if header == "" {
		return time.Second << attempt, response.StatusCode == http.StatusTooManyRequests
	}
	delay := time.Duration(-1)
	if seconds, err := strconv.Atoi(header); err == nil && seconds >= 0 {
		delay = time.Duration(seconds) * time.Second
	} else if date, err := http.ParseTime(header); err == nil {
		delay = max(0, time.Until(date))
	}
	return delay, delay >= 0 && delay <= maximumRetryAfter
}

func (transport *providerTransport) pause(host string, delay time.Duration) {
	transport.mu.Lock()
	defer transport.mu.Unlock()
	bucket := transport.bucket(host)
	if resume := time.Now().Add(delay); resume.After(bucket.resumeAt) {
		bucket.resumeAt = resume
	}
}

func (transport *providerTransport) bucket(host string) *providerBucket {
	bucket := transport.buckets[host]
	if bucket == nil {
		if len(transport.buckets) >= 64 {
			clear(transport.buckets)
		}
		bucket = &providerBucket{tokens: providerBurst, updated: time.Now()}
		transport.buckets[host] = bucket
	}
	return bucket
}

func (transport *providerTransport) wait(ctx context.Context, host string) error {
	for {
		transport.mu.Lock()
		bucket, now := transport.bucket(host), time.Now()
		bucket.tokens = min(providerBurst, bucket.tokens+now.Sub(bucket.updated).Seconds()*providerRequestsPerSecond)
		bucket.updated = now
		delay := bucket.resumeAt.Sub(now)
		if delay <= 0 && bucket.tokens >= 1 {
			bucket.tokens--
			transport.mu.Unlock()
			return nil
		}
		delay = max(delay, time.Duration((1-bucket.tokens)/providerRequestsPerSecond*float64(time.Second)))
		transport.mu.Unlock()
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
}
