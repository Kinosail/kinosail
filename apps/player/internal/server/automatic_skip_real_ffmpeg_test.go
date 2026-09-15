package server_test

import "testing"

func TestRealFFmpegAutomaticSkipServesFutureSegments(t *testing.T) {
	automaticSkipFixture.RealFFmpegAutomaticSkipServesFutureSegments(t)
}

func TestRealFFmpegAutomaticSkipCopiesPlayableVideo(t *testing.T) {
	automaticSkipFixture.RealFFmpegAutomaticSkipCopiesPlayableVideo(t)
}

func TestRealFFmpegAutomaticSkipPreservesTranscodedFrames(t *testing.T) {
	automaticSkipFixture.RealFFmpegAutomaticSkipPreservesTranscodedFrames(t)
}
