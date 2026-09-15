package server_test

import (
	"net/http"
	"testing"

	"github.com/MikeO7/kinosail-player/internal/server"
	"github.com/MikeO7/kinosail/packages/servertest"
)

func TestAlwaysOnSubtitlesUseASeparateTextTrackWithoutVideoTranscoding(t *testing.T) {
	fixture := servertest.SubtitlePreferenceFixture{NewHandler: func(media, data, ffprobe, ffmpeg string) http.Handler {
		return server.New(server.Config{MediaDir: media, DataDir: data, FFprobe: ffprobe, FFmpeg: ffmpeg})
	}}
	fixture.AlwaysOnSubtitlesRemainDirect(t)
}
