package server_test

import (
	"testing"

	"github.com/MikeO7/kinosail/packages/playback"
	"github.com/MikeO7/kinosail/packages/servertest"
)

var playbackPlanFixture = servertest.PlaybackPlanFixture{Policy: playback.DecisionPolicy{PreferCompatibleAudio: true}}

func TestDecidePlaybackGoldenPlans(t *testing.T) { playbackPlanFixture.DecidePlaybackGoldenPlans(t) }

func TestDirectFirstHasNoImplicitBitrateLimit(t *testing.T) {
	playbackPlanFixture.DirectFirstHasNoImplicitBitrateLimit(t)
}

func TestCompatibilityUsesSupportedAlternateAudio(t *testing.T) {
	playbackPlanFixture.CompatibilityUsesSupportedAlternateAudio(t)
}
