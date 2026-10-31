package server

import (
	"context"
	"errors"
	"net/http"
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
	mu      sync.Mutex
	states  map[string]subtitleProviderHealthState
	now     func() time.Time
	path    string
	saved   subtitleProviderHealthDiskState
	savedAt time.Time
}

const subtitleProviderHealthRefresh = 5 * time.Minute

const subtitleDailyLimitError = "Daily download limit reached"

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
	case http.StatusNotAcceptable:
		state.lastError = subtitleDailyLimitError
		state.nextRetry = state.reset
		if !state.nextRetry.After(now) {
			state.nextRetry = now.UTC().Truncate(24 * time.Hour).Add(24 * time.Hour)
		}
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
			if state.lastError == "Rate limit reached" || state.lastError == subtitleDailyLimitError {
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

func subtitleQuota(header http.Header, now time.Time, previous *int, previousReset time.Time) (*int, time.Time) {
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
	configured := map[string]bool{}
	for _, source := range provider.active().sources() {
		configured[source.name()] = source.configured()
	}
	return provider.health.views(configured)
}

func (provider *subtitleProvider) testCredentials(ctx context.Context) (int, int) {
	attempted, connected := 0, 0
	for _, source := range provider.active().sources() {
		if !source.configured() {
			continue
		}
		attempted++
		if source.checkCredentials(ctx) == nil {
			connected++
		}
	}
	return attempted, connected
}

func (registry *subtitleProviderHealthRegistry) observeFile(name string, response *http.Response, requestErr error) {
	if response == nil || response.StatusCode != http.StatusNotFound && response.StatusCode != http.StatusGone {
		registry.observe(name, response, requestErr)
		return
	}
	registry.mu.Lock()
	defer registry.mu.Unlock()
	now, state := registry.now(), registry.states[name]
	state.remaining, state.reset = subtitleQuota(response.Header, now, state.remaining, state.reset)
	state.lastSuccess, state.lastError, state.nextRetry, state.failures = now, "", time.Time{}, 0
	registry.states[name] = state
	registry.saveLocked()
}
