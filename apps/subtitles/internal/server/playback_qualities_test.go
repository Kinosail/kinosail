package server_test

import "testing"

func TestAdaptiveQualitiesPreserveSourceShapeWithoutUpscaling(t *testing.T) {
	playbackPlanFixture.AdaptiveQualitiesPreserveSourceShapeWithoutUpscaling(t)
}

func TestAdaptiveQualitiesUseConventionalTierForCroppedVideo(t *testing.T) {
	playbackPlanFixture.AdaptiveQualitiesUseConventionalTierForCroppedVideo(t)
}
