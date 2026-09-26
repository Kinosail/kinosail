package server

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

type subtitleProviderHealth struct {
	Name          string `json:"name"`
	State         string `json:"state"`
	Configured    bool   `json:"configured"`
	Tested        bool   `json:"tested"`
	LastSuccess   string `json:"lastSuccess,omitempty"`
	Remaining     *int   `json:"remaining,omitempty"`
	ResetTime     string `json:"resetTime,omitempty"`
	LastSafeError string `json:"lastSafeError,omitempty"`
	NextRetry     string `json:"nextRetry,omitempty"`
}

type subtitleProviderHealthState struct {
	lastSuccess, reset, nextRetry time.Time
	remaining                     *int
	lastError                     string
	failures                      int
}

type subtitleProviderHealthRegistry struct {
	mu     sync.Mutex
	states map[string]subtitleProviderHealthState
	now    func() time.Time
	path   string
}

type subtitleProviderHealthDiskState struct {
	Version   int                                    `json:"version"`
	Providers map[string]subtitleProviderHealthEntry `json:"providers"`
}

type subtitleProviderHealthEntry struct {
	LastSuccess int64  `json:"last_success,omitempty"`
	Reset       int64  `json:"reset,omitempty"`
	NextRetry   int64  `json:"next_retry,omitempty"`
	Remaining   *int   `json:"remaining,omitempty"`
	LastError   string `json:"last_error,omitempty"`
	Failures    int    `json:"failures,omitempty"`
}

func newSubtitleProviderHealthRegistry(dataDir ...string) *subtitleProviderHealthRegistry {
	registry := &subtitleProviderHealthRegistry{states: make(map[string]subtitleProviderHealthState), now: time.Now}
	if len(dataDir) > 0 && dataDir[0] != "" {
		registry.path = filepath.Join(dataDir[0], "subtitle_provider_health.json")
		registry.load()
	}
	return registry
}

func (registry *subtitleProviderHealthRegistry) before(name string) error {
	registry.mu.Lock()
	defer registry.mu.Unlock()
	state := registry.states[name]
	if !state.nextRetry.IsZero() && registry.now().Before(state.nextRetry) {
		return errors.New("subtitle provider retry is delayed")
	}
	return nil
}

func (registry *subtitleProviderHealthRegistry) reset(name string) {
	registry.mu.Lock()
	defer registry.mu.Unlock()
	delete(registry.states, name)
	registry.saveLocked()
}

func (registry *subtitleProviderHealthRegistry) observe(name string, response *http.Response, requestErr error) {
	registry.mu.Lock()
	defer registry.mu.Unlock()
	now, state := registry.now(), registry.states[name]
	if response != nil {
		state.remaining, state.reset = subtitleQuota(response.Header, now, state.remaining, state.reset)
	}
	status := 0
	if response != nil {
		status = response.StatusCode
	}
	if requestErr == nil && status >= 200 && status <= 299 {
		state.lastSuccess, state.lastError, state.nextRetry, state.failures = now, "", time.Time{}, 0
		registry.states[name] = state
		registry.saveLocked()
		return
	}
	state.failures++
	state.lastError = subtitleSafeProviderError(status)
	switch status {
	case http.StatusUnauthorized, http.StatusForbidden:
		state.nextRetry = now.Add(24 * time.Hour)
	case http.StatusTooManyRequests:
		state.nextRetry = subtitleRetryTime(response.Header, now)
		if state.nextRetry.IsZero() {
			state.nextRetry = now.Add(time.Hour)
		}
	default:
		delay := subtitleProviderBackoff(name, state.failures)
		state.nextRetry = now.Add(delay)
	}
	registry.states[name] = state
	registry.saveLocked()
}

func subtitleProviderBackoff(name string, failures int) time.Duration {
	base := min(time.Duration(1<<min(max(failures-1, 0), 6))*time.Minute, time.Hour)
	seed := failures * 17
	for _, character := range name {
		seed += int(character)
	}
	jitterPercent := seed%21 - 10
	return base + time.Duration(int64(base)*int64(jitterPercent)/100)
}

func (registry *subtitleProviderHealthRegistry) load() { //nolint:cyclop,gocognit // Persisted provider state requires explicit validation of every bounded field.
	file, err := os.Open(registry.path) //nolint:gosec // The path is fixed installation-owned state.
	if err != nil {
		return
	}
	defer file.Close()
	decoder := json.NewDecoder(io.LimitReader(file, 64<<10))
	decoder.DisallowUnknownFields()
	var disk subtitleProviderHealthDiskState
	if decoder.Decode(&disk) != nil || decoder.Decode(&struct{}{}) != io.EOF || disk.Version != 1 || len(disk.Providers) > 3 {
		return
	}
	now := time.Now()
	for name, entry := range disk.Providers {
		if !oneOf(name, "SubDL", "OpenSubtitles", "SubSource") || entry.LastSuccess < 0 || entry.LastSuccess > now.Add(24*time.Hour).Unix() || entry.Reset < 0 || entry.Reset > now.Add(8*24*time.Hour).Unix() || entry.NextRetry < 0 || entry.NextRetry > now.Add(8*24*time.Hour).Unix() || entry.Failures < 0 || entry.Failures > 1000 || !oneOf(entry.LastError, "", "Credentials rejected", "Rate limit reached", "Provider connection failed", "Provider unavailable") || entry.Remaining != nil && (*entry.Remaining < 0 || *entry.Remaining > 1_000_000_000) {
			return
		}
		registry.states[name] = subtitleProviderHealthState{lastSuccess: subtitleTimeFromUnix(entry.LastSuccess), reset: subtitleTimeFromUnix(entry.Reset), nextRetry: subtitleTimeFromUnix(entry.NextRetry), remaining: entry.Remaining, lastError: entry.LastError, failures: entry.Failures}
	}
}

func (registry *subtitleProviderHealthRegistry) saveLocked() {
	if registry.path == "" {
		return
	}
	disk := subtitleProviderHealthDiskState{Version: 1, Providers: make(map[string]subtitleProviderHealthEntry, len(registry.states))}
	for name, state := range registry.states {
		disk.Providers[name] = subtitleProviderHealthEntry{subtitleUnix(state.lastSuccess), subtitleUnix(state.reset), subtitleUnix(state.nextRetry), state.remaining, state.lastError, state.failures}
	}
	_ = saveDurableJSON(registry.path, disk)
}

func subtitleUnix(value time.Time) int64 {
	if value.IsZero() {
		return 0
	}
	return value.Unix()
}

func subtitleTimeFromUnix(value int64) time.Time {
	if value == 0 {
		return time.Time{}
	}
	return time.Unix(value, 0)
}

func (registry *subtitleProviderHealthRegistry) views(configured map[string]bool) []subtitleProviderHealth {
	registry.mu.Lock()
	defer registry.mu.Unlock()
	now := registry.now()
	result := make([]subtitleProviderHealth, 0, 3)
	for _, name := range []string{"SubDL", "OpenSubtitles", "SubSource"} {
		state := registry.states[name]
		view := subtitleProviderHealth{Name: name, State: "unavailable", Configured: configured[name], Tested: !state.lastSuccess.IsZero(), Remaining: state.remaining, LastSafeError: state.lastError}
		switch {
		case !view.Configured:
			view.State = "not configured"
		case !state.nextRetry.IsZero() && now.Before(state.nextRetry):
			view.State = "unavailable"
			if state.lastError == "Rate limit reached" {
				view.State = "limited"
			}
			view.NextRetry = state.nextRetry.UTC().Format(time.RFC3339)
		case view.Tested:
			view.State = "connected"
		default:
			view.State = "untested"
		}
		if view.Tested {
			view.LastSuccess = state.lastSuccess.UTC().Format(time.RFC3339)
		}
		if !state.reset.IsZero() {
			view.ResetTime = state.reset.UTC().Format(time.RFC3339)
		}
		result = append(result, view)
	}
	return result
}

func subtitleQuota(header http.Header, now time.Time, previous *int, previousReset time.Time) (*int, time.Time) { //nolint:cyclop,gocognit // Both standard and provider-specific quota forms share strict bounds.
	remaining := previous
	for _, key := range []string{"RateLimit-Remaining", "X-RateLimit-Remaining"} {
		if value, err := strconv.Atoi(strings.TrimSpace(header.Get(key))); err == nil && value >= 0 && value <= 1_000_000_000 {
			remaining = &value
			break
		}
	}
	reset := previousReset
	for _, key := range []string{"RateLimit-Reset", "X-RateLimit-Reset"} {
		if value := strings.TrimSpace(header.Get(key)); value != "" {
			if seconds, err := strconv.ParseInt(value, 10, 64); err == nil && seconds >= 0 {
				candidate := time.Time{}
				if seconds >= now.Unix()-24*60*60 && seconds <= now.Add(7*24*time.Hour).Unix() {
					candidate = time.Unix(seconds, 0)
				} else if seconds <= 7*24*60*60 {
					candidate = now.Add(time.Duration(seconds) * time.Second)
				}
				if candidate.After(now) {
					reset = candidate
				}
			}
			break
		}
	}
	return remaining, reset
}

func subtitleRetryTime(header http.Header, now time.Time) time.Time {
	value := strings.TrimSpace(header.Get("Retry-After"))
	if seconds, err := strconv.Atoi(value); err == nil && seconds >= 0 && seconds <= 7*24*60*60 {
		return now.Add(time.Duration(seconds) * time.Second)
	}
	if parsed, err := http.ParseTime(value); err == nil && parsed.After(now) && parsed.Before(now.Add(7*24*time.Hour)) {
		return parsed
	}
	_, reset := subtitleQuota(header, now, nil, time.Time{})
	return reset
}

func subtitleSafeProviderError(status int) string {
	switch status {
	case http.StatusUnauthorized, http.StatusForbidden:
		return "Credentials rejected"
	case http.StatusTooManyRequests:
		return "Rate limit reached"
	case 0:
		return "Provider connection failed"
	default:
		return "Provider unavailable"
	}
}

func (provider *subtitleProvider) healthViews() []subtitleProviderHealth {
	provider = provider.active()
	return provider.health.views(map[string]bool{"SubDL": provider.subDLConfigured(), "OpenSubtitles": provider.open.configured(), "SubSource": provider.subsource.configured()})
}

func (provider *subtitleProvider) testCredentials(ctx context.Context) (int, int) {
	provider = provider.active()
	attempted, connected := 0, 0
	if provider.subDLConfigured() {
		attempted++
		endpoint, _ := url.Parse(strings.TrimRight(provider.config.URL, "/"))
		endpoint.Path = strings.TrimSuffix(endpoint.Path, "/api/v1") + "/api/v2/me"
		query := endpoint.Query()
		query.Set("api_key", provider.config.APIKey)
		endpoint.RawQuery = query.Encode()
		var response map[string]any
		if provider.json(ctx, endpoint.String(), &response) == nil {
			connected++
		}
	}
	if provider.open.configured() {
		attempted++
		if _, _, err := provider.open.session(ctx); err == nil {
			connected++
		}
	}
	if provider.subsource.configured() {
		attempted++
		endpoint, err := provider.subsource.endpoint("languages")
		var response map[string]any
		if err == nil && provider.subsource.requestJSON(ctx, endpoint.String(), &response) == nil {
			connected++
		}
	}
	return attempted, connected
}
