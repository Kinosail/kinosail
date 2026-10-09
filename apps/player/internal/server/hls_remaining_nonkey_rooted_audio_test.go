//go:build linux

package server

import (
	"context"
	"crypto/sha256"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/MikeO7/kinosail/packages/workload"
)

// A public ready response cannot expose which private file objects supplied the
// AAC association or prove their shared operation budget. These actual fixtures
// retain that acquisition gap; they do not grant production cache admission.
func TestRemainingNonKeyRootedPrivateSourceAudio(t *testing.T) {
	ctx, ffmpeg, ffprobe, sources := remainingNonKeyRootedTools(t)
	for _, source := range sources {
		for _, offset := range []float64{12.5, 13.5, 18.2} {
			t.Run(fmt.Sprintf("%s-%.1f", filepath.Ext(source), offset), func(t *testing.T) {
				fixture := remainingNonKeyRootedFixtureFor(t, ctx, ffmpeg, ffprobe, source, offset)
				remainingNonKeyRootedPositive(t, ctx, fixture)
			})
		}
	}
}

func remainingNonKeyRootedPositive(t *testing.T, ctx context.Context, fixture remainingNonKeyRootedFixture) {
	t.Helper()
	manager := fixture.manager
	releaseWork, err := manager.workloads.Acquire(ctx, workload.Playback)
	if err != nil {
		t.Fatal("rooted private capacity-one owner reservation")
	}
	defer releaseWork()
	remainingNonKeyCollectorReserved(t, manager)
	before := remainingNonKeyRootedFDCount(t, manager.cache)
	started := time.Now()
	proof, facts, err := manager.measureCopiedHLSPrivateSourceAudio(ctx, fixture.item, fixture.recipe, fixture.policy, fixture.directory)
	elapsed := time.Since(started)
	if err != nil || facts == nil {
		t.Fatal("nonkey rooted private source-clock acquisition missing")
	}
	remainingNonKeyPrivateAudioCorrespondence(t, facts, fixture.item.Path, fixture.recipe.offset,
		sha256.Sum256(fixture.initialization), sha256.Sum256(fixture.first), elapsed)
	remainingNonKeyCollectorResult(t, proof, err, elapsed, facts.FirstPacket, fixture.recipe.offset, facts.OriginalMediaTime)
	_, expected := remainingNonKeyCollectorExpected(t, fixture.item.Path, fixture.recipe.offset)
	remainingNonKeyCollectorCorrespondence(t, proof, expected)
	if remainingNonKeyRootedFDCount(t, manager.cache) != before {
		t.Fatal("nonkey rooted private acquisition leaked a file or root descriptor")
	}
	fixture.unchanged(t)
	remainingNonKeyCollectorReserved(t, manager)
	remainingNonKeyRootedLeaseReleased(t, manager)
	t.Logf("nonkey rooted-private container=%s offset=%.1f elapsed_ns=%d init_sha=%x first_sha=%x first_aac_sha=%x source_clock=%x joined_fixture=true cache_metadata=0 descriptors_released=true",
		filepath.Ext(fixture.item.Path), fixture.recipe.offset, elapsed.Nanoseconds(),
		facts.Initialization, facts.First, facts.FirstPacket, proof.SourceClock)
}
