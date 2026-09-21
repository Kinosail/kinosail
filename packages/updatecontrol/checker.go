package updatecontrol

import (
	"context"
	"crypto/rand"
	"errors"
	"io"
	"math/big"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	updateCheckInterval = 24 * time.Hour
	updateCheckJitter   = 6 * time.Hour
)

// Status is the application-facing state of the update checker.
type Status struct {
	Automatic      bool   `json:"automatic"`
	State          string `json:"state"`
	CurrentVersion string `json:"currentVersion"`
	LatestVersion  string `json:"latestVersion,omitempty"`
	CheckedAt      string `json:"checkedAt,omitempty"`
	ReleaseURL     string `json:"releaseUrl"`
	Manager        View   `json:"manager"`
}

// ReleaseSource returns the newest public release and its cache validator.
type ReleaseSource interface {
	Latest(context.Context, string) (version, etag string, unchanged bool, err error)
}

// ReleaseSourceFunc adapts a function to ReleaseSource.
type ReleaseSourceFunc func(context.Context, string) (string, string, bool, error)

// Latest calls the adapted release source.
func (function ReleaseSourceFunc) Latest(ctx context.Context, etag string) (string, string, bool, error) {
	return function(ctx, etag)
}

// CheckerConfig supplies app adapters to the shared checker.
type CheckerConfig struct {
	Manager        *Store
	Source         ReleaseSource
	CurrentVersion string
	Automatic      func() bool
	SaveAutomatic  func(bool) error
}

// Checker checks releases, requests approved installs, and schedules checks.
type Checker struct {
	mu            sync.RWMutex
	checkMu       sync.Mutex
	manager       *Store
	source        ReleaseSource
	current       string
	status        Status
	etag          string
	trigger       chan struct{}
	automatic     func() bool
	saveAutomatic func(bool) error
	now           func() time.Time
	random        io.Reader
	delay         func(io.Reader) time.Duration
}

// NewChecker validates its adapters before creating a checker.
func NewChecker(config CheckerConfig) (*Checker, error) {
	if config.Source == nil || config.Automatic == nil || config.SaveAutomatic == nil || config.CurrentVersion == "" || len(config.CurrentVersion) > 64 || strings.HasPrefix(config.CurrentVersion, "sha-") && !commitVersion(config.CurrentVersion) || strings.ContainsAny(config.CurrentVersion, "\r\n") {
		return nil, errors.New("invalid update checker configuration")
	}
	return &Checker{
		manager: config.Manager, source: config.Source, current: config.CurrentVersion,
		status:  Status{State: "not-checked", CurrentVersion: config.CurrentVersion, ReleaseURL: GitHubReleasesURL},
		trigger: make(chan struct{}, 1), automatic: config.Automatic, saveAutomatic: config.SaveAutomatic,
		now: time.Now, random: rand.Reader, delay: randomUpdateDelay,
	}, nil
}

// View returns a snapshot with current preference and update-manager state.
func (checker *Checker) View() Status {
	checker.mu.RLock()
	status := checker.status
	checker.mu.RUnlock()
	status.Automatic = checker.automatic()
	if commitVersion(checker.current) {
		status.State = "container-managed"
		status.Automatic = false
	}
	if checker.manager != nil {
		status.Manager, _ = checker.manager.View(checker.current)
	}
	return status
}

// Available reports whether a newer release is ready for approval.
func (checker *Checker) Available() bool { return checker.View().State == "available" }

// SetAutomatic persists the preference and triggers a prompt check when enabled.
func (checker *Checker) SetAutomatic(enabled bool) error {
	if err := checker.saveAutomatic(enabled); err != nil {
		return err
	}
	if enabled {
		select {
		case checker.trigger <- struct{}{}:
		default:
		}
	}
	return nil
}

// Check refreshes the public release state.
func (checker *Checker) Check(ctx context.Context) Status {
	if commitVersion(checker.current) {
		return checker.View()
	}
	checker.checkMu.Lock()
	defer checker.checkMu.Unlock()
	checker.mu.RLock()
	etag, prior := checker.etag, checker.status.LatestVersion
	checker.mu.RUnlock()
	latest, nextETag, unchanged, err := checker.source.Latest(ctx, etag)
	if unchanged {
		latest = prior
		if latest == "" {
			err = errors.New("GitHub returned an empty cached release")
		}
	}
	status := Status{State: "unavailable", CurrentVersion: checker.current, ReleaseURL: GitHubReleasesURL, CheckedAt: checker.now().UTC().Format(time.RFC3339)}
	if errors.Is(err, errNoPublicRelease) {
		status.State = "no-release"
	}
	if err == nil {
		status.LatestVersion = latest
		status.State = "current"
		if newerRelease(checker.current, latest) {
			status.State = "available"
		}
	}
	checker.mu.Lock()
	checker.status = status
	if nextETag != "" {
		checker.etag = nextETag
	}
	checker.mu.Unlock()
	if status.State == "available" && checker.automatic() && checker.manager != nil {
		_, _ = checker.manager.Request(status.LatestVersion)
	}
	return checker.View()
}

// CheckAndRequest refreshes releases and requests the available version.
func (checker *Checker) CheckAndRequest(ctx context.Context) (Status, error) {
	checker.Check(ctx)
	return checker.RequestAvailable()
}

// RequestAvailable requests the last checked available release.
func (checker *Checker) RequestAvailable() (Status, error) {
	status := checker.View()
	if status.State != "available" || checker.manager == nil {
		return status, nil
	}
	_, err := checker.manager.Request(status.LatestVersion)
	return checker.View(), err
}

// Schedule checks immediately when enabled and then at a randomized daily interval.
func (checker *Checker) Schedule(ctx context.Context) {
	go func() {
		for {
			if checker.automatic() {
				checker.Check(ctx)
			}
			timer := time.NewTimer(checker.delay(checker.random))
			select {
			case <-ctx.Done():
				timer.Stop()
				return
			case <-checker.trigger:
				timer.Stop()
			case <-timer.C:
			}
		}
	}()
}

func randomUpdateDelay(random io.Reader) time.Duration {
	value, err := rand.Int(random, big.NewInt(int64(updateCheckJitter)))
	if err != nil {
		return updateCheckInterval
	}
	return updateCheckInterval - updateCheckJitter/2 + time.Duration(value.Int64())
}

func newerRelease(current, latest string) bool {
	currentParts, currentOK := releaseVersion(current)
	latestParts, latestOK := releaseVersion(latest)
	if !currentOK || !latestOK {
		return false
	}
	for index := range currentParts {
		if latestParts[index] != currentParts[index] {
			return latestParts[index] > currentParts[index]
		}
	}
	return false
}

func releaseVersion(value string) ([3]uint64, bool) {
	var result [3]uint64
	if len(value) > 64 {
		return result, false
	}
	core := strings.SplitN(strings.TrimPrefix(value, "v"), "-", 2)[0]
	parts := strings.Split(core, ".")
	if len(parts) != len(result) {
		return result, false
	}
	for index, part := range parts {
		if part == "" || len(part) > 1 && part[0] == '0' {
			return result, false
		}
		parsed, err := strconv.ParseUint(part, 10, 64)
		if err != nil {
			return result, false
		}
		result[index] = parsed
	}
	return result, true
}

// Commit images are updated by the container deployment, never by tag releases.
func commitVersion(value string) bool {
	return len(value) == 44 && strings.HasPrefix(value, "sha-") && strings.Trim(value[4:], "0123456789abcdef") == ""
}
