package server_test

import "testing"

func TestCompatiblePlaybackGeneratesAnAlignedSeekableSuffix(t *testing.T) {
	transcodeFixture().CompatiblePlaybackGeneratesAnAlignedSeekableSuffix(t, "2940000", "2940")
}

func TestCompatiblePlaybackRejectsInvalidSeekOffsetsWithoutEncoding(t *testing.T) {
	transcodeFixture().CompatiblePlaybackRejectsInvalidSeekOffsetsWithoutEncoding(t)
}
