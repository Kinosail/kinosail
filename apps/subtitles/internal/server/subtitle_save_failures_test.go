package server

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestSubtitleUpgradeRejectsCancellationAndMissingSidecar(t *testing.T) {
	t.Parallel()
	manager, item, _, current := subtitleReviewFixture(t)
	provider := newSubtitleProvider(SubtitleConfig{URL: "https://example.invalid", APIKey: "test"}, t.TempDir(), t.TempDir(), manager.index, nil, "")
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if changed, err := provider.upgradeSidecar(ctx, item, "en"); changed || !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled upgrade=%v %v", changed, err)
	}
	saved, err := os.ReadFile(subtitleSidecarPath(item, "en"))
	if err != nil || string(saved) != string(current) {
		t.Fatal("canceled upgrade changed sidecar")
	}
	if err := os.Remove(subtitleSidecarPath(item, "en")); err != nil {
		t.Fatal(err)
	}
	if changed, err := provider.upgradeSidecar(t.Context(), item, "en"); changed || err == nil {
		t.Fatalf("missing sidecar=%v %v", changed, err)
	}
}

func TestSubtitleEditStorageFailuresPreserveSidecar(t *testing.T) {
	t.Parallel()
	for _, scenario := range []string{"original", "backup"} {
		t.Run(scenario, func(t *testing.T) { assertSubtitleEditStorageFailure(t, scenario) })
	}
}

func assertSubtitleEditStorageFailure(t *testing.T, scenario string) {
	t.Helper()
	manager, item, request, current := subtitleReviewFixture(t)
	if scenario == "original" {
		path := filepath.Dir(manager.provider.originalPath(subtitleFingerprint(current)))
		if err := os.WriteFile(path, []byte("blocked"), 0o600); err != nil {
			t.Fatal(err)
		}
	} else if err := os.Mkdir(subtitleSidecarPath(item, "en")+".kinosail.bak", 0o700); err != nil {
		t.Fatal(err)
	}
	input := subtitleEdit{Language: "en", Fingerprint: subtitleFingerprint(current), Text: "1\n00:00:01,000 --> 00:00:03,000\nEdited words\n"}
	_, status, err := manager.applySubtitleEdit(request, item.ID, input)
	if err == nil || status < 400 {
		t.Fatalf("blocked save=%d %v", status, err)
	}
	saved, err := os.ReadFile(subtitleSidecarPath(item, "en"))
	if err != nil || string(saved) != string(current) {
		t.Fatal("failed save changed sidecar")
	}
}
