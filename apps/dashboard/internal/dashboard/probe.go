package dashboard

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

const (
	probeTimeout          = 5 * time.Second
	slowThreshold         = 1500 * time.Millisecond
	probeConcurrency      = 4
	probeBatchConcurrency = 3
)

// Prober performs bounded server-side reachability observations.
type Prober struct {
	service  *Service
	client   *http.Client
	interval time.Duration
	runSlot  chan struct{}
	slots    chan struct{}
	flightMu sync.Mutex
	flights  map[probeFlightKey]*probeFlight
}

type probeFlight struct {
	done    chan struct{}
	health  Health
	err     error
	cancel  context.CancelFunc
	waiters int
}

// NewProber creates a redirect-free client with destination validation.
func NewProber(service *Service, interval time.Duration, publicHosts []string) *Prober {
	if interval < 15*time.Second {
		interval = 30 * time.Second
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	transport.DialContext = newProbeDialer(publicHosts, net.DefaultResolver.LookupIPAddr)
	transport.DisableCompression = true
	transport.ResponseHeaderTimeout = probeTimeout
	transport.MaxResponseHeaderBytes = 16 << 10
	transport.MaxIdleConns = probeConcurrency
	transport.MaxConnsPerHost = 2
	client := &http.Client{Transport: transport, Timeout: probeTimeout, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	return &Prober{service: service, client: client, interval: interval, runSlot: make(chan struct{}, 1), slots: make(chan struct{}, probeConcurrency), flights: make(map[probeFlightKey]*probeFlight)}
}

// Run checks configured applications until the lifecycle ends.
func (prober *Prober) Run(ctx context.Context) {
	prober.ProbeAll(ctx)
	ticker := time.NewTicker(prober.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			prober.ProbeAll(ctx)
		}
	}
}

// ProbeAll checks one isolated copy of the current applications.
func (prober *Prober) ProbeAll(ctx context.Context) ProbeBatch {
	apps := prober.service.appsForProbe()
	select {
	case prober.runSlot <- struct{}{}:
		defer func() { <-prober.runSlot }()
	case <-ctx.Done():
		return ProbeBatch{Requested: len(apps), Canceled: true}
	}
	jobs := make(chan App)
	var workers sync.WaitGroup
	var completed int
	var completedMu sync.Mutex
	for range min(probeBatchConcurrency, len(apps)) {
		workers.Add(1)
		go func() {
			defer workers.Done()
			prober.probeJobs(ctx, jobs, &completed, &completedMu)
		}()
	}
	for _, app := range apps {
		select {
		case <-ctx.Done():
			close(jobs)
			workers.Wait()
			return ProbeBatch{Requested: len(apps), Completed: completed, Canceled: true}
		case jobs <- app:
		}
	}
	close(jobs)
	workers.Wait()
	return ProbeBatch{Requested: len(apps), Completed: completed, Canceled: ctx.Err() != nil}
}

func (prober *Prober) probeJobs(ctx context.Context, jobs <-chan App, completed *int, completedMu *sync.Mutex) {
	for app := range jobs {
		if ctx.Err() != nil {
			continue
		}
		if _, err := prober.ProbeOne(ctx, app.ID); err == nil {
			completedMu.Lock()
			(*completed)++
			completedMu.Unlock()
		}
	}
}

func (prober *Prober) check(ctx context.Context, address string) Health {
	if _, err := normalizeURL("healthUrl", address, true); err != nil {
		return Health{State: "unavailable", CheckedAt: time.Now().UTC(), Explanation: "Health address is invalid"}
	}
	started := time.Now()
	requestCtx, cancel := context.WithTimeout(ctx, probeTimeout)
	defer cancel()
	request, err := http.NewRequestWithContext(requestCtx, http.MethodGet, address, nil)
	if err != nil {
		return failedHealth(started, "Request could not be created")
	}
	request.Header.Set("User-Agent", "Kinosail-Dashboard/1")
	request.Header.Set("Range", "bytes=0-0")
	response, err := prober.client.Do(request)
	if err != nil {
		return failedHealth(started, explainProbeError(err))
	}
	_, _ = io.CopyN(io.Discard, response.Body, 1)
	_ = response.Body.Close()
	latency := time.Since(started)
	state := "reachable"
	explanation := fmt.Sprintf("HTTP %d responded", response.StatusCode)
	switch {
	case response.StatusCode >= 300 && response.StatusCode < 400:
		explanation = fmt.Sprintf("HTTP %d responded; redirect not followed", response.StatusCode)
	case response.StatusCode >= 500:
		state = "degraded"
	case latency >= slowThreshold:
		state = "slow"
	}
	return Health{State: state, LatencyMS: latency.Milliseconds(), HTTPStatus: response.StatusCode, CheckedAt: time.Now().UTC(), Explanation: explanation}
}

type lookupIP func(context.Context, string) ([]net.IPAddr, error)

func newProbeDialer(publicHosts []string, lookup lookupIP) func(context.Context, string, string) (net.Conn, error) {
	allowed := make(map[string]bool, len(publicHosts))
	for _, host := range publicHosts {
		allowed[strings.ToLower(strings.TrimSuffix(host, "."))] = true
	}
	return func(ctx context.Context, network, address string) (net.Conn, error) {
		return safeDialContext(ctx, network, address, allowed, lookup)
	}
}

func safeDialContext(ctx context.Context, network, address string, allowed map[string]bool, lookup lookupIP) (net.Conn, error) {
	ctx, cancel := context.WithTimeout(ctx, probeTimeout)
	defer cancel()
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, errors.New("invalid probe destination")
	}
	if forbiddenHost(host) {
		return nil, errors.New("probe destination is blocked")
	}
	addresses, err := lookup(ctx, host)
	if err != nil || len(addresses) == 0 {
		return nil, errors.New("probe destination could not be resolved")
	}
	if err := validateResolvedProbeAddresses(host, addresses, allowed); err != nil {
		return nil, err
	}
	dialer := &net.Dialer{Timeout: probeTimeout, KeepAlive: 30 * time.Second}
	return dialProbeAddresses(ctx, network, port, addresses, dialer.DialContext)
}

func validateResolvedProbeAddresses(host string, addresses []net.IPAddr, allowed map[string]bool) error {
	if len(addresses) == 0 || len(addresses) > 64 {
		return errors.New("probe address count is invalid")
	}
	hasPublic := false
	for _, resolved := range addresses {
		if forbiddenIP(resolved.IP) || resolved.Zone != "" {
			return errors.New("probe destination is blocked")
		}
		hasPublic = hasPublic || !resolved.IP.IsPrivate() && !resolved.IP.IsLoopback()
	}
	if hasPublic && !allowed[strings.ToLower(strings.TrimSuffix(host, "."))] {
		return errors.New("public probe destination is not allowed")
	}
	return nil
}

func forbiddenHost(host string) bool {
	host = strings.ToLower(strings.TrimSuffix(host, "."))
	return host == "metadata.google.internal" || host == "metadata" || strings.HasSuffix(host, ".internal.metadata")
}

func forbiddenIP(ip net.IP) bool {
	if ip == nil {
		return true
	}
	if ipv4 := ip.To4(); ipv4 != nil {
		ip = ipv4
	}
	if ip.IsUnspecified() || ip.IsMulticast() || ip.IsLinkLocalMulticast() || ip.IsLinkLocalUnicast() {
		return true
	}
	return ip.Equal(net.ParseIP("169.254.169.254")) || ip.Equal(net.ParseIP("100.100.100.200"))
}

func failedHealth(started time.Time, explanation string) Health {
	return Health{State: "unavailable", LatencyMS: time.Since(started).Milliseconds(), CheckedAt: time.Now().UTC(), Explanation: explanation}
}

func explainProbeError(err error) string {
	var urlError *url.Error
	if errors.As(err, &urlError) {
		err = urlError.Err
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return "Check timed out"
	}
	var operation *net.OpError
	if errors.As(err, &operation) {
		if operation.Timeout() {
			return "Check timed out"
		}
		if operation.Op == "dial" {
			return "Service did not accept a connection"
		}
	}
	return "Service could not be reached"
}

func (service *Service) appsForProbe() []App {
	service.mu.RLock()
	defer service.mu.RUnlock()
	apps := make([]App, 0, len(service.board.Apps))
	for _, app := range service.board.Apps {
		if app.CheckEnabled {
			apps = append(apps, app)
		}
	}
	return apps
}

func (service *Service) setHealth(id string, generation uint64, health Health) {
	service.mu.Lock()
	defer service.mu.Unlock()
	if service.healthGeneration[id] == generation {
		service.health[id] = health
	}
}
