package servertest

import (
	"testing"

	"github.com/MikeO7/kinosail/packages/playback"
)

func PlaybackPresentationNamesEveryTransformPrecisely(t *testing.T, presentation func(playback.PlaybackPlan) (string, string)) {
	t.Helper()
	t.Parallel()
	for _, test := range []struct {
		mode, reason, label, description string
	}{
		{"direct", "direct-preferred", "Direct Play", "Original video and audio. No conversion."},
		{"remux", "container-unsupported", "Remux", "Repackages the original video and audio without conversion."},
		{"audio-transcode", "audio-codec-unsupported", "Transcoding audio", "Keeps the original video. Converts only the selected audio."},
		{"transcode", "video-codec-unsupported", "Transcoding video", "This device cannot decode the original video."},
		{"transcode", "bitrate-exceeds-limit", "Transcoding video", "The original exceeds an explicit streaming limit."},
	} {
		label, description := presentation(playback.PlaybackPlan{Mode: test.mode, Reason: test.reason, VideoCodec: "h264"})
		if label != test.label || description != test.description {
			t.Errorf("%s/%s = %q, %q; want %q, %q", test.mode, test.reason, label, description, test.label, test.description)
		}
	}
}
