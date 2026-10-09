//go:build linux

package server

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/MikeO7/kinosail/packages/servertest"
	"github.com/MikeO7/kinosail/packages/workload"
)

// Actual joined processes witness revocation and private changes after snapshot
// acquisition. Parser-only controls cannot cover this cross-operation interval.
func TestRemainingNonKeyRootedPrivateOperationBounds(t *testing.T) {
	ctx, ffmpeg, ffprobe, sources := remainingNonKeyRootedTools(t)
	base := remainingNonKeyRootedFixtureFor(t, ctx, ffmpeg, ffprobe, sources[0], 12.5)
	first := remainingNonKeyPrivateAudioPad(t, base.first, len(base.first)+32)
	for _, name := range []string{"revoked-root", "replaced-source", "private-restored-mtime", "inherited-deadline"} {
		t.Run(name, func(t *testing.T) {
			fixture := remainingNonKeyRootedFixtureFrom(t, ctx, ffmpeg, ffprobe, sources[0], 12.5, base.initialization, first)
			remainingNonKeyRootedOperationBound(t, ctx, fixture, ffmpeg, ffprobe, name)
		})
	}
}

func remainingNonKeyRootedOperationBound(t *testing.T, ctx context.Context, fixture remainingNonKeyRootedFixture, ffmpeg, ffprobe, name string) {
	t.Helper()
	manager := fixture.manager
	releaseWork, err := manager.workloads.Acquire(ctx, workload.Playback)
	if err != nil {
		t.Fatal("rooted private bounded owner reservation")
	}
	defer releaseWork()
	remainingNonKeyCollectorReserved(t, manager)
	marker := filepath.Join(t.TempDir(), "owned-pids")
	originalSource, err := os.Stat(fixture.item.Path)
	if err != nil {
		t.Fatal("rooted private original source witness")
	}
	originalFirst, err := os.Stat(filepath.Join(fixture.directory, "segment-00000.m4s"))
	if err != nil {
		t.Fatal("rooted private original first witness")
	}
	probeAction, normalizedAction := remainingNonKeyRootedProcessActions(t, fixture, name)
	manager.probe.executable = remainingNonKeyRootedProcess(t, ffprobe, marker, probeAction)
	manager.ffmpeg = remainingNonKeyRootedProcess(t, ffmpeg, marker, normalizedAction)
	request, release := remainingNonKeyRootedOperationContext(t, ctx, manager, name)
	defer release()
	revocation, finishRevocation := remainingNonKeyRootedRevocation(request, manager, marker, name)
	deadline, bounded := request.Deadline()
	started := time.Now()
	proof, facts, err := manager.measureCopiedHLSPrivateSourceAudio(request, fixture.item, fixture.recipe, fixture.policy, fixture.directory)
	elapsed := time.Since(started)
	finishRevocation()
	remainingNonKeyRootedOperationResult(t, proof, facts, err, elapsed, <-revocation)
	if name == "inherited-deadline" && (!bounded || time.Now().After(deadline)) {
		t.Fatal("nonkey rooted private operation exceeded the inherited original deadline")
	}
	remainingNonKeyRootedProcessWitness(t, marker, name)
	remainingNonKeyRootedOperationFiles(t, fixture, name, originalSource, originalFirst)
	remainingNonKeyCollectorReserved(t, manager)
	release()
	remainingNonKeyRootedLeaseReleased(t, manager)
	if remainingNonKeyRootedFDCount(t, manager.cache) != 0 {
		t.Fatal("nonkey bounded private collection leaked a retained descriptor")
	}
	t.Logf("nonkey rooted-private-operation case=%s elapsed_ns=%d processes_settled=true descriptors_released=true metadata_unpublished=true",
		name, elapsed.Nanoseconds())
}

func remainingNonKeyRootedOperationContext(t *testing.T, ctx context.Context, manager *hlsManager, name string) (context.Context, func()) {
	t.Helper()
	if name != "inherited-deadline" {
		return ctx, func() {}
	}
	request, release, err := manager.copiedHLSClockAdmission(ctx)
	if err != nil {
		t.Fatal("rooted private inherited metadata admission")
	}
	once := sync.Once{}
	joinedRelease := func() { once.Do(release) }
	t.Cleanup(joinedRelease)
	timer := time.NewTimer(900 * time.Millisecond)
	defer timer.Stop()
	select {
	case <-timer.C:
	case <-request.Done():
		t.Fatal("rooted private inherited lease expired before the operation")
	}
	return request, joinedRelease
}

func remainingNonKeyRootedRevocation(ctx context.Context, manager *hlsManager, marker, name string) (<-chan bool, func()) {
	request, cancel := context.WithCancel(ctx)
	if name == "revoked-root" {
		return remainingNonKeyCollectorRevoke(request, manager, marker), cancel
	}
	done := make(chan bool, 1)
	done <- true
	return done, cancel
}

func remainingNonKeyRootedProcessActions(t *testing.T, fixture remainingNonKeyRootedFixture, name string) (string, string) {
	t.Helper()
	switch name {
	case "revoked-root":
		return "sleep 0.2\n", ""
	case "inherited-deadline":
		return "sleep 0.7\n", "sleep 0.7\n"
	case "replaced-source":
		before, err := os.Stat(fixture.item.Path)
		if err != nil {
			t.Fatal("rooted private source replacement identity")
		}
		t.Cleanup(func() {
			remainingNonKeyRestoreOwnedSource(t, fixture.item.Path, fixture.item.Path+".retained", before, fixture.source)
		})
		source, backup := copiedRecoveryQuote(fixture.item.Path), copiedRecoveryQuote(fixture.item.Path+".retained")
		return "", "mv " + source + " " + backup + "\ncp -p " + backup + " " + source + "\n"
	case "private-restored-mtime":
		first := copiedRecoveryQuote(filepath.Join(fixture.directory, "segment-00000.m4s"))
		backup := copiedRecoveryQuote(filepath.Join(fixture.directory, "first-mtime"))
		action := "cp -p " + first + " " + backup + "\nprintf '\\001' | dd of=" + first +
			" bs=1 seek=" + strconv.Itoa(len(fixture.first)-1) + " count=1 conv=notrunc status=none\n" +
			"touch -r " + backup + " " + first + "\nrm " + backup + "\n"
		return "", action
	}
	t.Fatal("rooted private unknown process control")
	return "", ""
}

func remainingNonKeyRootedProcess(t *testing.T, executable, marker, action string) string {
	t.Helper()
	body := "#!/bin/sh\nset -eu\nprintf '%s\\n' $$ >> " + copiedRecoveryQuote(marker) + "\n" +
		action + "exec " + copiedRecoveryQuote(executable) + " \"$@\"\n"
	name := filepath.Join(t.TempDir(), "owned-tool")
	servertest.WriteExecutable(t, name, body)
	return name
}

func remainingNonKeyRootedProcessWitness(t *testing.T, marker, name string) {
	t.Helper()
	data := remainingNonKeyCollectorRead(t, marker, 1024)
	pids := strings.Fields(string(data))
	expected := 2
	if name == "revoked-root" {
		expected = 1
	}
	if len(pids) != expected {
		t.Fatal("nonkey rooted private process controls did not execute the expected joined tools")
	}
	for _, pid := range pids {
		copiedRecoveryAssertStopped(t, []byte(pid))
	}
}

func remainingNonKeyRootedOperationResult(t *testing.T, proof *copiedHLSAudioProof, facts *copiedHLSPrivateAudioFacts, err error, elapsed time.Duration, revoked bool) {
	t.Helper()
	if !revoked || err == nil || proof != nil || facts != nil || elapsed > 2*time.Second {
		t.Fatal("nonkey invalid rooted private operation acquired identity or exceeded shared budget")
	}
}
