package server_test

import "testing"

func TestAdaptiveQualitiesPreserveSourceShapeWithoutUpscaling(t *testing.T) {
	playbackPlanFixture.AdaptiveQualitiesPreserveSourceShapeWithoutUpscaling(t)
}

func TestAdaptiveQualitiesUseConventionalTierForCroppedVideo(t *testing.T) {
	playbackPlanFixture.AdaptiveQualitiesUseConventionalTierForCroppedVideo(t)
}

func TestAdaptiveQualitiesMatchFFmpegEvenRounding(t *testing.T) {
	playbackPlanFixture.AdaptiveQualitiesMatchFFmpegEvenRounding(t)
}
