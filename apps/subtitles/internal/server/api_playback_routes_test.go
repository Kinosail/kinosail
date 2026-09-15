package server_test

import (
	"net/http"
	"testing"

	"github.com/MikeO7/kinosail-subtitles/internal/server"
	"github.com/MikeO7/kinosail/packages/servertest"
)

var playbackAPIContracts = servertest.PlaybackAPIFixture{
	New: func(config servertest.PlaybackAPIConfig) http.Handler {
		return server.New(server.Config{MediaDir: config.MediaDir, DataDir: config.DataDir, FFprobe: config.FFprobe, FFmpeg: config.FFmpeg, RequireAuth: config.RequireAuth, ProbeHardware: config.ProbeHardware, HardwareDevices: config.HardwareDevices, HardwareOS: config.HardwareOS, HardwareArch: config.HardwareArch})
	},
	SignIn: signInTestProfile,
}

func TestPlaybackAPIExposesSelectableAudioSources(t *testing.T) {
	playbackAPIContracts.PlaybackAPIExposesSelectableAudioSources(t)
}

func TestPlaybackAPIUsesTheSmallestClientSupportedTransformation(t *testing.T) {
	playbackAPIContracts.PlaybackAPIUsesTheSmallestClientSupportedTransformation(t)
}

func TestPlaybackAPIRejectsInvalidCodecEvidenceBeforeProbingMedia(t *testing.T) {
	playbackAPIContracts.PlaybackAPIRejectsInvalidCodecEvidenceBeforeProbingMedia(t)
}

func TestPlaybackAPIExposesSourceAwareAdaptiveQualities(t *testing.T) {
	t.Parallel()
	assertPlaybackProbe(t, `{"streams":[{"codec_type":"video","codec_name":"hevc","width":1920,"height":1080,"r_frame_rate":"24/1"},{"codec_type":"audio","codec_name":"aac"}],"format":{"format_name":"matroska","bit_rate":"10000000"}}`,
		`"qualities":[{"label":"360p","width":640,"height":360,"bitrate":493000,"frameRate":24},{"label":"432p","width":768,"height":432,"bitrate":1228000,"frameRate":24},{"label":"540p","width":960,"height":540,"bitrate":2128000,"frameRate":24},{"label":"720p","width":1280,"height":720,"bitrate":3128000,"frameRate":24},{"label":"1080p","width":1920,"height":1080,"bitrate":6128000,"frameRate":24}]`)
}

func TestPlaybackAPIUsesConventionalTierForCroppedVideo(t *testing.T) {
	t.Parallel()
	assertPlaybackProbe(t, `{"streams":[{"codec_type":"video","codec_name":"hevc","width":1920,"height":804},{"codec_type":"audio","codec_name":"aac"}],"format":{"format_name":"matroska"}}`,
		`"label":"1080p","width":1920,"height":804`)
}

func assertPlaybackProbe(t *testing.T, probe string, expected ...string) {
	t.Helper()
	playbackAPIContracts.AssertProbe(t, probe, expected...)
}
