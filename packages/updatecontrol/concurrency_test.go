package updatecontrol

import (
	"context"
	"reflect"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestConcurrentChecksSerializeReleaseAccessAndKeepSnapshotsValid(t *testing.T) {
	const checks = 24
	var active, calls, invalidSnapshots, maximum atomic.Int32
	start := make(chan struct{})
	checker := mustChecker(t, CheckerConfig{
		CurrentVersion: "v1.0.0",
		Automatic:      func() bool { return false },
		SaveAutomatic:  func(bool) error { return nil },
		Source: ReleaseSourceFunc(func(context.Context, string) (string, string, bool, error) {
			current := active.Add(1)
			defer active.Add(-1)
			for observed := maximum.Load(); current > observed && !maximum.CompareAndSwap(observed, current); observed = maximum.Load() {
			}
			calls.Add(1)
			time.Sleep(time.Millisecond)
			return "v1.1.0", "", false, nil
		}),
	})

	var group sync.WaitGroup
	for range checks {
		group.Add(1)
		go func() {
			defer group.Done()
			<-start
			status := checker.Check(t.Context())
			if status.State != "available" || status.LatestVersion != "v1.1.0" {
				invalidSnapshots.Add(1)
			}
		}()
	}
	close(start)
	group.Wait()
	if calls.Load() != checks || maximum.Load() != 1 || invalidSnapshots.Load() != 0 {
		t.Fatalf("concurrent checks: calls=%d maximum=%d invalid snapshots=%d", calls.Load(), maximum.Load(), invalidSnapshots.Load())
	}
}

func assertPlayerPlan(t *testing.T, got Plan, err error, requestID string) {
	t.Helper()
	manifest := "https://github.com/Kinosail/kinosail/releases/download/player-v1.2.4/kinosail-release.json"
	want := Plan{
		SchemaVersion: PlanSchemaVersion, StateSchema: 1, ConfigurationSchema: 1,
		CurrentVersion: "v1.2.3", Automatic: true, RequestID: requestID,
		RequestedAt: "2026-08-27T12:00:00Z", TargetVersion: "v1.2.4",
		ManifestURL: manifest, ManifestSignatureURL: manifest + ".sigstore.json",
		SignatureIdentity: PlayerPolicy(1, 1).signatureIdentity, RequireSignature: true,
		BackupBeforeInstall: true, DeferWhilePlaying: true, RollbackOnUnhealthy: true,
		Recovery: RecoveryPlan{
			BackupCommand: []string{"backup"}, VerifyCommand: []string{"backup", "verify"},
			RestoreCommand: []string{"restore"}, Format: "kinosail-backup-v1-encrypted",
			IncludesSecrets: true, RequiresKey: true, RestoreOnRollback: true,
		},
	}
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("plan = %#v, %v", got, err)
	}
}
