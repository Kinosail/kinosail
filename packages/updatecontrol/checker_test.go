package updatecontrol

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestNewCheckerRejectsInvalidAdaptersBeforeEffects(t *testing.T) {
	t.Parallel()
	var sourceCalls, preferenceCalls atomic.Int32
	source := ReleaseSourceFunc(func(context.Context, string) (string, string, bool, error) {
		sourceCalls.Add(1)
		return "v1.0.0", "", false, nil
	})
	automatic := func() bool { preferenceCalls.Add(1); return true }
	save := func(bool) error { preferenceCalls.Add(1); return nil }
	valid := CheckerConfig{Source: source, CurrentVersion: "v1.0.0", Automatic: automatic, SaveAutomatic: save}
	for name, mutate := range map[string]func(*CheckerConfig){
		"source":          func(config *CheckerConfig) { config.Source = nil },
		"current missing": func(config *CheckerConfig) { config.CurrentVersion = "" },
		"current long":    func(config *CheckerConfig) { config.CurrentVersion = strings.Repeat("x", 65) },
		"current newline": func(config *CheckerConfig) { config.CurrentVersion = "v1.0.0\nextra" },
		"preference":      func(config *CheckerConfig) { config.Automatic = nil },
		"save":            func(config *CheckerConfig) { config.SaveAutomatic = nil },
	} {
		t.Run(name, func(t *testing.T) {
			config := valid
			mutate(&config)
			if checker, err := NewChecker(config); err == nil || checker != nil {
				t.Fatalf("invalid config created checker %#v, err=%v", checker, err)
			}
		})
	}
	if sourceCalls.Load() != 0 || preferenceCalls.Load() != 0 {
		t.Fatalf("invalid configuration caused effects: source=%d preference=%d", sourceCalls.Load(), preferenceCalls.Load())
	}
}

func TestCheckerComparesVersionsAndRetainsCachedRelease(t *testing.T) { //nolint:cyclop // One sequence verifies first-check and cached-check invariants together.
	t.Parallel()
	var calls atomic.Int32
	checker := mustChecker(t, CheckerConfig{
		CurrentVersion: "v1.2.9-nox.4",
		Automatic:      func() bool { return true },
		SaveAutomatic:  func(bool) error { return nil },
		Source: ReleaseSourceFunc(func(_ context.Context, etag string) (string, string, bool, error) {
			if calls.Add(1) == 1 {
				if etag != "" {
					t.Fatalf("first etag = %q", etag)
				}
				return "v1.3.0", `"one"`, false, nil
			}
			if etag != `"one"` {
				t.Fatalf("cached etag = %q", etag)
			}
			return "", `"two"`, true, nil
		}),
	})
	checker.now = func() time.Time { return time.Date(2026, 9, 4, 12, 0, 0, 0, time.UTC) }
	if status := checker.View(); !status.Automatic || status.State != "not-checked" {
		t.Fatalf("initial status = %#v", status)
	}
	for range 2 {
		status := checker.Check(t.Context())
		if status.State != "available" || status.LatestVersion != "v1.3.0" || status.CheckedAt != "2026-09-04T12:00:00Z" {
			t.Fatalf("checked status = %#v", status)
		}
	}
	if !checker.Available() || calls.Load() != 2 || checker.etag != `"two"` {
		t.Fatalf("cache state: available=%t calls=%d etag=%q", checker.Available(), calls.Load(), checker.etag)
	}
}

func TestReleaseComparisonRejectsAmbiguousVersions(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		current string
		latest  string
		newer   bool
	}{
		{current: "v1.2.9", latest: "v1.3.0", newer: true},
		{current: "1.2.9", latest: "v1.3.0", newer: true},
		{current: "v1.3.0-nox.4", latest: "v2.0.0", newer: true},
		{current: "v1.3.0", latest: "v1.3.0"},
		{current: "v2.0.0", latest: "v1.9.9"},
		{current: "dev", latest: "v9.0.0"},
		{current: "v1.2.3", latest: "v1.02.4"},
		{current: "v1.2.3", latest: "v1.2"},
		{current: "v1.2.3", latest: "v1..4"},
		{current: "v1.2.3", latest: strings.Repeat("9", 65)},
	} {
		if got := newerRelease(test.current, test.latest); got != test.newer {
			t.Errorf("newerRelease(%q, %q) = %t, want %t", test.current, test.latest, got, test.newer)
		}
	}
}

func TestCheckerMapsReleaseFailuresWithoutRequestingAnUpdate(t *testing.T) {
	t.Parallel()
	for name, sourceErr := range map[string]error{
		"missing": errNoPublicRelease,
		"failed":  errors.New("network failed"),
	} {
		t.Run(name, func(t *testing.T) {
			manager, err := New(nil, PlayerPolicy(1, 1))
			if err != nil {
				t.Fatal(err)
			}
			checker := mustChecker(t, CheckerConfig{
				Manager: manager, CurrentVersion: "v1.0.0", Automatic: func() bool { return true }, SaveAutomatic: func(bool) error { return nil },
				Source: ReleaseSourceFunc(func(context.Context, string) (string, string, bool, error) { return "", "", false, sourceErr }),
			})
			status := checker.Check(t.Context())
			want := "unavailable"
			if errors.Is(sourceErr, errNoPublicRelease) {
				want = "no-release"
			}
			if status.State != want || status.Manager.RequestID != "" {
				t.Fatalf("failure status = %#v, want %q", status, want)
			}
		})
	}

	checker := mustChecker(t, CheckerConfig{
		CurrentVersion: "v1.0.0", Automatic: func() bool { return false }, SaveAutomatic: func(bool) error { return nil },
		Source: ReleaseSourceFunc(func(context.Context, string) (string, string, bool, error) { return "", "", true, nil }),
	})
	if status := checker.Check(t.Context()); status.State != "unavailable" {
		t.Fatalf("empty cache status = %#v", status)
	}
}

func TestAutomaticAndManualChecksPinTheAvailableRelease(t *testing.T) { //nolint:cyclop,gocognit // Paired policy modes share the same immutable-target assertions.
	t.Parallel()
	for _, automatic := range []bool{false, true} {
		t.Run(map[bool]string{false: "manual", true: "automatic"}[automatic], func(t *testing.T) {
			manager, err := New(nil, PlayerPolicy(1, 1))
			if err != nil {
				t.Fatal(err)
			}
			manager.random = bytes.NewReader(bytes.Repeat([]byte{7}, 32))
			checker := mustChecker(t, CheckerConfig{
				Manager: manager, CurrentVersion: "v1.0.0", Automatic: func() bool { return automatic }, SaveAutomatic: func(bool) error { return nil },
				Source: ReleaseSourceFunc(func(context.Context, string) (string, string, bool, error) { return "v1.1.0", "", false, nil }),
			})
			status := checker.Check(t.Context())
			if !automatic && status.Manager.RequestID != "" {
				t.Fatalf("manual check requested update: %#v", status)
			}
			if !automatic {
				status, err = checker.RequestAvailable()
			}
			if err != nil || status.State != "available" || status.Manager.Status != "requested" || status.Manager.TargetVersion != "v1.1.0" || status.Manager.RequestID == "" {
				t.Fatalf("request status = %#v, err=%v", status, err)
			}
			plan, planErr := manager.Plan("v1.0.0", automatic)
			if planErr != nil || plan.TargetVersion != "v1.1.0" || plan.RequestID == "" {
				t.Fatalf("plan = %#v, err=%v", plan, planErr)
			}
		})
	}
}

func TestRequestAvailableRequiresManagerAndCompletedAvailableCheck(t *testing.T) {
	t.Parallel()
	checker := mustChecker(t, CheckerConfig{
		CurrentVersion: "v1.0.0", Automatic: func() bool { return false }, SaveAutomatic: func(bool) error { return nil },
		Source: ReleaseSourceFunc(func(context.Context, string) (string, string, bool, error) { return "v1.0.0", "", false, nil }),
	})
	if status, err := checker.RequestAvailable(); err != nil || status.State != "not-checked" {
		t.Fatalf("unchecked request = %#v, %v", status, err)
	}
	if status, err := checker.CheckAndRequest(t.Context()); err != nil || status.State != "current" || status.Manager.RequestID != "" {
		t.Fatalf("current request = %#v, %v", status, err)
	}
}

func TestSetAutomaticPersistsBeforeTriggering(t *testing.T) { //nolint:cyclop // One sequence proves failed, disabled, enabled, and coalesced persistence behavior.
	t.Parallel()
	var saved atomic.Int32
	checker := mustChecker(t, CheckerConfig{
		CurrentVersion: "v1.0.0", Automatic: func() bool { return false },
		SaveAutomatic: func(bool) error { saved.Add(1); return errors.New("save failed") },
		Source:        ReleaseSourceFunc(func(context.Context, string) (string, string, bool, error) { return "v1.0.0", "", false, nil }),
	})
	if err := checker.SetAutomatic(true); err == nil || saved.Load() != 1 || len(checker.trigger) != 0 {
		t.Fatalf("failed save: err=%v saves=%d triggers=%d", err, saved.Load(), len(checker.trigger))
	}
	checker.saveAutomatic = func(bool) error { saved.Add(1); return nil }
	if err := checker.SetAutomatic(false); err != nil || len(checker.trigger) != 0 {
		t.Fatalf("manual save: err=%v triggers=%d", err, len(checker.trigger))
	}
	if err := checker.SetAutomatic(true); err != nil || len(checker.trigger) != 1 || saved.Load() != 3 {
		t.Fatalf("automatic save: err=%v saves=%d triggers=%d", err, saved.Load(), len(checker.trigger))
	}
	if err := checker.SetAutomatic(true); err != nil || len(checker.trigger) != 1 {
		t.Fatalf("duplicate trigger was not coalesced: err=%v triggers=%d", err, len(checker.trigger))
	}
}

func TestScheduleHonorsPreferenceAndTriggeredChecks(t *testing.T) {
	t.Parallel()
	var enabled atomic.Bool
	calls := make(chan struct{}, 2)
	checker := mustChecker(t, CheckerConfig{
		CurrentVersion: "v1.0.0", Automatic: enabled.Load, SaveAutomatic: func(value bool) error { enabled.Store(value); return nil },
		Source: ReleaseSourceFunc(func(context.Context, string) (string, string, bool, error) {
			calls <- struct{}{}
			return "v1.0.0", "", false, nil
		}),
	})
	checker.random = errorReader{}
	ctx, cancel := context.WithCancel(t.Context())
	checker.Schedule(ctx)
	select {
	case <-calls:
		t.Fatal("disabled scheduler checked a release")
	case <-time.After(20 * time.Millisecond):
	}
	if err := checker.SetAutomatic(true); err != nil {
		t.Fatal(err)
	}
	select {
	case <-calls:
	case <-time.After(time.Second):
		t.Fatal("enabled scheduler did not check a release")
	}
	cancel()
}

func TestScheduleChecksAgainWhenItsTimerExpires(t *testing.T) {
	t.Parallel()
	calls := make(chan struct{}, 2)
	checker := mustChecker(t, CheckerConfig{
		CurrentVersion: "v1.0.0", Automatic: func() bool { return true }, SaveAutomatic: func(bool) error { return nil },
		Source: ReleaseSourceFunc(func(context.Context, string) (string, string, bool, error) {
			calls <- struct{}{}
			return "v1.0.0", "", false, nil
		}),
	})
	checker.delay = func(io.Reader) time.Duration { return time.Millisecond }
	ctx, cancel := context.WithCancel(t.Context())
	checker.Schedule(ctx)
	for range 2 {
		select {
		case <-calls:
		case <-time.After(time.Second):
			t.Fatal("scheduled check did not run")
		}
	}
	cancel()
}

func TestRandomUpdateDelayIsBoundedAndHasSafeFallback(t *testing.T) {
	t.Parallel()
	if got := randomUpdateDelay(errorReader{}); got != updateCheckInterval {
		t.Fatalf("fallback delay = %v", got)
	}
	got := randomUpdateDelay(bytes.NewReader(bytes.Repeat([]byte{0}, 16)))
	minimum := updateCheckInterval - updateCheckJitter/2
	if got < minimum || got >= minimum+updateCheckJitter {
		t.Fatalf("random delay = %v", got)
	}
}

func mustChecker(t *testing.T, config CheckerConfig) *Checker {
	t.Helper()
	checker, err := NewChecker(config)
	if err != nil {
		t.Fatal(err)
	}
	return checker
}
