package updatecontrol

import (
	"context"
	"strings"
	"sync/atomic"
	"testing"
)

func TestCommitImagesDoNotQueryOrInstallVersionReleases(t *testing.T) {
	t.Parallel()
	var calls atomic.Int32
	manager, err := New(nil, PlayerPolicy(1, 1))
	if err != nil {
		t.Fatal(err)
	}
	checker := mustChecker(t, CheckerConfig{
		Manager: manager, CurrentVersion: "sha-" + strings.Repeat("a", 40),
		Automatic: func() bool { return true }, SaveAutomatic: func(bool) error { return nil },
		Source: ReleaseSourceFunc(func(context.Context, string) (string, string, bool, error) {
			calls.Add(1)
			return "v99.0.0", "", false, nil
		}),
	})
	for _, status := range []Status{checker.View(), checker.Check(t.Context())} {
		if status.State != "container-managed" || status.Automatic || status.Manager.RequestID != "" || status.LatestVersion != "" {
			t.Fatalf("container status = %#v", status)
		}
	}
	status, err := checker.CheckAndRequest(t.Context())
	if err != nil || calls.Load() != 0 || status.Manager.RequestID != "" {
		t.Fatalf("unexpected release side effects: %#v, %v, calls=%d", status, err, calls.Load())
	}
}

func TestCommitImageVersionsRejectMalformedInputBeforeEffects(t *testing.T) {
	t.Parallel()
	for _, version := range []string{"sha-", "sha-" + strings.Repeat("a", 39), "sha-" + strings.Repeat("a", 41), "sha-" + strings.Repeat("A", 40), "sha-" + strings.Repeat("g", 40)} {
		checker, err := NewChecker(CheckerConfig{
			CurrentVersion: version,
			Source: ReleaseSourceFunc(func(context.Context, string) (string, string, bool, error) {
				t.Fatal("source called")
				return "", "", false, nil
			}),
			Automatic:     func() bool { t.Fatal("preference read"); return false },
			SaveAutomatic: func(bool) error { t.Fatal("preference changed"); return nil },
		})
		if checker != nil || err == nil {
			t.Fatalf("accepted malformed version %q", version)
		}
	}
}
