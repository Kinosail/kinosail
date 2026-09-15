package updatecontrol

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestPolicyRequestAndAdapterReportShareOneContract(t *testing.T) { //nolint:cyclop // One contract covers policy, adapter, and report validation.
	db := newMemoryDocumentStore()
	store, err := New(db, PlayerPolicy(1, 1))
	if err != nil {
		t.Fatal(err)
	}
	store.now = func() time.Time { return time.Date(2026, 8, 27, 12, 0, 0, 0, time.UTC) }
	store.random = bytes.NewReader(bytes.Repeat([]byte{0x2a}, 16))
	requestID, err := store.Request("v1.2.4")
	if err != nil {
		t.Fatal(err)
	}
	plan, err := store.Plan("v1.2.3", true)
	assertPlayerPlan(t, plan, err, requestID)
	if err := store.Report(Report{Adapter: "windows", State: "available", CurrentVersion: "v1.2.3", AvailableVersion: "v1.2.4", RequestID: requestID, CheckedAt: time.Date(2026, 8, 27, 12, 2, 0, 0, time.UTC)}); err != nil {
		t.Fatal(err)
	}
	view, err := store.View("v1.2.3")
	if err != nil || view.Status != "available" || view.RequestID != requestID || view.AvailableVersion != "v1.2.4" {
		t.Fatalf("available view = %#v, %v", view, err)
	}
	if err := store.Report(Report{Adapter: "windows", State: "installing", Phase: PhaseDownloading, CurrentVersion: "v1.2.3", AvailableVersion: "v1.2.4", RequestID: requestID, CheckedAt: time.Date(2026, 8, 27, 12, 3, 0, 0, time.UTC)}); err != nil {
		t.Fatal(err)
	}
	view, err = store.View("v1.2.3")
	if err != nil || view.Status != "installing" || view.Phase != PhaseDownloading || view.RequestID != requestID {
		t.Fatalf("installing view = %#v, %v", view, err)
	}
	if err := store.Report(Report{Adapter: "windows", State: "current", CurrentVersion: "v1.2.4", RequestID: requestID, CheckedAt: time.Date(2026, 8, 27, 12, 5, 0, 0, time.UTC)}); err != nil {
		t.Fatal(err)
	}
	view, err = store.View("v1.2.4")
	if err != nil || view.Status != "current" || view.Adapter != "windows" || view.RequestID != "" || view.CurrentVersion != "v1.2.4" {
		t.Fatalf("view = %#v, %v", view, err)
	}
}

func TestApplicationPoliciesKeepIndependentReleaseIdentities(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name, wantURL, wantIdentity string
		policy                      Policy
	}{
		{name: "player", policy: PlayerPolicy(2, 3), wantURL: "https://github.com/MikeO7/kinosail/releases/download/player-v1.2.3/kinosail-release.json", wantIdentity: `^https://github\.com/MikeO7/kinosail/\.github/workflows/player-release\.yml@refs/tags/player-v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$`},
		{name: "subtitles", policy: SubtitlesPolicy(4, 5), wantURL: "https://github.com/MikeO7/kinosail/releases/download/subtitles-v1.2.3/kinosail-release.json", wantIdentity: `^https://github\.com/MikeO7/kinosail/\.github/workflows/subtitles-release\.yml@refs/tags/subtitles-v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$`},
	} {
		t.Run(test.name, func(t *testing.T) {
			manifest, signature := releaseManifestURLs("v1.2.3", test.policy)
			if manifest != test.wantURL || signature != manifest+".sigstore.json" || test.policy.signatureIdentity != test.wantIdentity {
				t.Fatalf("release policy = %#v, %q, %q", test.policy, manifest, signature)
			}
		})
	}
}

func TestNewRejectsInvalidPolicyBeforeLoadingState(t *testing.T) {
	t.Parallel()
	for _, policy := range []Policy{PlayerPolicy(0, 1), PlayerPolicy(1, 1001), {}} {
		db := newMemoryDocumentStore()
		if _, err := New(db, policy); err == nil {
			t.Fatalf("invalid policy accepted: %#v", policy)
		}
		if db.loads != 0 {
			t.Fatalf("invalid policy loaded state %d times", db.loads)
		}
	}
}

func TestDocumentStoreFailuresDoNotMutateUpdateState(t *testing.T) {
	t.Parallel()
	failedLoad := newMemoryDocumentStore()
	failedLoad.loadErr = errors.New("load failed")
	if _, err := New(failedLoad, PlayerPolicy(1, 1)); err == nil {
		t.Fatal("document load failure accepted")
	}
	db := newMemoryDocumentStore()
	store, err := New(db, PlayerPolicy(1, 1))
	if err != nil {
		t.Fatal(err)
	}
	store.random = bytes.NewReader(bytes.Repeat([]byte{7}, 16))
	db.saveErr = errors.New("save failed")
	if _, err = store.Request("v1.2.3"); err == nil {
		t.Fatal("document save failure accepted")
	}
	db.saveErr = nil
	view, err := store.View("v1.2.2")
	if err != nil || view.RequestID != "" || view.Status != "unavailable" {
		t.Fatalf("failed save mutated state: %#v, %v", view, err)
	}
}

func TestInvalidReportsAndFailedPersistenceHaveNoSideEffects(t *testing.T) {
	store, err := New(nil, PlayerPolicy(1, 1))
	if err != nil {
		t.Fatal(err)
	}
	store.random = errorReader{}
	if _, err := store.Request("v1.2.3"); err == nil {
		t.Fatal("request accepted unavailable randomness")
	}
	before, _ := store.View("dev")
	for _, report := range []Report{
		{CheckedAt: time.Now()},
		{Adapter: "portable", State: "current", CheckedAt: time.Now()},
		{Adapter: "linux", State: "unknown", CheckedAt: time.Now()},
		{Adapter: "linux", State: "available", CheckedAt: time.Now()},
		{Adapter: "linux", State: "current", CheckedAt: time.Now()},
		{Adapter: "linux", State: "failed", CheckedAt: time.Now()},
		{Adapter: "linux", State: "failed", Message: string(bytes.Repeat([]byte{'x'}, 241)), CheckedAt: time.Now()},
		{Adapter: "linux", State: "failed", Message: "network unavailable"},
		{Adapter: "linux", State: "installing", Phase: "unknown", CheckedAt: time.Now()},
		{Adapter: "linux", State: "installing", Phase: PhaseDownloading, ErrorCode: ErrorDownloadFailed, CheckedAt: time.Now()},
		{Adapter: "linux", State: "current", CurrentVersion: "v1.0.0", Phase: PhaseStarting, CheckedAt: time.Now()},
		{Adapter: "linux", State: "failed", Message: "failed", ErrorCode: "unknown", CheckedAt: time.Now()},
	} {
		if err := store.Report(report); err == nil {
			t.Fatalf("accepted report %#v", report)
		}
	}
	after, _ := store.View("dev")
	if before != after {
		t.Fatalf("state changed: before=%#v after=%#v", before, after)
	}
}

func TestStaleReportCannotCompleteAnotherRequest(t *testing.T) {
	store, _ := New(nil, PlayerPolicy(1, 1))
	store.random = bytes.NewReader(bytes.Repeat([]byte{1}, 16))
	requestID, err := store.Request("v1.0.1")
	if err != nil {
		t.Fatal(err)
	}
	err = store.Report(Report{Adapter: "docker", State: "current", CurrentVersion: "dev", RequestID: "02020202020202020202020202020202", CheckedAt: time.Now()})
	if err == nil {
		t.Fatal("stale report was accepted")
	}
	view, _ := store.View("dev")
	if view.RequestID != requestID || view.Status != "requested" {
		t.Fatalf("pending request changed: %#v", view)
	}
}

func TestRequestRequiresOneImmutableReleaseTarget(t *testing.T) {
	store, _ := New(nil, PlayerPolicy(1, 1))
	store.random = bytes.NewReader(bytes.Repeat([]byte{3}, 16))
	for _, target := range []string{"", "latest", "v1.2", "v1.02.3", "v1.2.3-beta"} {
		if _, err := store.Request(target); err == nil {
			t.Fatalf("accepted target %q", target)
		}
	}
	requestID, err := store.Request("v1.2.3")
	if err != nil {
		t.Fatal(err)
	}
	if retried, err := store.Request("v1.2.3"); err != nil || retried != requestID {
		t.Fatalf("retry = %q, %v", retried, err)
	}
	if _, err := store.Request("v1.2.4"); err == nil {
		t.Fatal("pending target was replaced")
	}
}

func TestPendingRequestRejectsMissingTargetAndOutOfOrderReports(t *testing.T) {
	store, _ := New(nil, PlayerPolicy(1, 1))
	store.random = bytes.NewReader(bytes.Repeat([]byte{4}, 16))
	requestID, _ := store.Request("v2.0.0")
	now := time.Now()
	for _, report := range []Report{
		{Adapter: "linux", State: "installing", AvailableVersion: "v2.0.0", CheckedAt: now},
		{Adapter: "linux", State: "available", AvailableVersion: "v2.1.0", RequestID: requestID, CheckedAt: now},
		{Adapter: "linux", State: "current", CurrentVersion: "v1.0.0", RequestID: requestID, CheckedAt: now},
	} {
		if err := store.Report(report); err == nil {
			t.Fatalf("accepted report %#v", report)
		}
	}
	view, _ := store.View("v1.0.0")
	if view.Status != "requested" || view.TargetVersion != "v2.0.0" {
		t.Fatalf("request changed: %#v", view)
	}
	if err := store.Report(Report{Adapter: "linux", State: "installing", Phase: PhaseBackingUp, AvailableVersion: "v2.0.0", RequestID: requestID, CheckedAt: now}); err != nil {
		t.Fatal(err)
	}
	if err := store.Report(Report{Adapter: "linux", State: "checking", AvailableVersion: "v2.0.0", RequestID: requestID, CheckedAt: now}); err == nil {
		t.Fatal("installing update moved back to checking")
	}
}

func TestFailedInstallRemainsRecoverableUntilRollbackCompletes(t *testing.T) { //nolint:cyclop // One failure scenario covers recovery state transitions.
	store, _ := New(nil, PlayerPolicy(1, 1))
	store.random = bytes.NewReader(bytes.Repeat([]byte{5}, 16))
	requestID, _ := store.Request("v3.0.0")
	now := time.Now()
	if err := store.Report(Report{Adapter: "macos", State: "installing", Phase: PhaseCheckingHealth, CurrentVersion: "v2.0.0", AvailableVersion: "v3.0.0", RequestID: requestID, CheckedAt: now}); err != nil {
		t.Fatal(err)
	}
	if err := store.Report(Report{Adapter: "macos", State: "failed", Phase: PhaseCheckingHealth, ErrorCode: ErrorHealthCheckFailed, CurrentVersion: "v2.0.0", AvailableVersion: "v3.0.0", Message: "health check failed", RequestID: requestID, CheckedAt: now}); err != nil {
		t.Fatal(err)
	}
	failed, _ := store.View("v2.0.0")
	if failed.Status != "failed" || failed.Phase != PhaseCheckingHealth || failed.ErrorCode != ErrorHealthCheckFailed || failed.RequestID != requestID || failed.TargetVersion != "v3.0.0" {
		t.Fatalf("failed update lost recovery state: %#v", failed)
	}
	if err := store.Report(Report{Adapter: "macos", State: "rolled-back", ErrorCode: ErrorHealthCheckFailed, CurrentVersion: "v2.0.0", Message: "restored v2.0.0", RequestID: requestID, CheckedAt: now}); err != nil {
		t.Fatal(err)
	}
	recovered, _ := store.View("v2.0.0")
	if recovered.Status != "rolled-back" || recovered.RequestID != "" || recovered.TargetVersion != "" {
		t.Fatalf("rollback did not finish recovery: %#v", recovered)
	}
}

func TestLegacyTargetlessRequestIsClearedBeforeUse(t *testing.T) {
	db := newMemoryDocumentStore()
	if err := db.SaveJSON(document, State{RequestID: strings.Repeat("a", 32), RequestedAt: "2026-08-27T12:00:00Z"}); err != nil {
		t.Fatal(err)
	}
	store, err := New(db, PlayerPolicy(1, 1))
	if err != nil {
		t.Fatal(err)
	}
	view, err := store.View("v1.0.0")
	if err != nil || view.RequestID != "" || view.TargetVersion != "" || view.Status != "failed" || view.Message == "" {
		t.Fatalf("legacy request = %#v, %v", view, err)
	}
}

type errorReader struct{}

func (errorReader) Read([]byte) (int, error) { return 0, errors.New("unavailable") }

type memoryDocumentStore struct {
	documents map[string][]byte
	loads     int
	loadErr   error
	saveErr   error
}

func newMemoryDocumentStore() *memoryDocumentStore {
	return &memoryDocumentStore{documents: make(map[string][]byte)}
}

func (store *memoryDocumentStore) LoadJSON(name string, target any) (bool, error) {
	store.loads++
	if store.loadErr != nil {
		return false, store.loadErr
	}
	data, ok := store.documents[name]
	if !ok {
		return false, nil
	}
	return true, json.Unmarshal(data, target)
}

func (store *memoryDocumentStore) SaveJSON(name string, value any) error {
	if store.saveErr != nil {
		return store.saveErr
	}
	data, err := json.Marshal(value)
	if err == nil {
		store.documents[name] = data
	}
	return err
}
