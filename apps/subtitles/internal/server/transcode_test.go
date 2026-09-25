package server_test

import (
	"net/http"
	"testing"

	"github.com/MikeO7/kinosail-subtitles/internal/server"
	"github.com/MikeO7/kinosail/packages/servertest"
)

func transcodeFixture() servertest.TranscodeFixture {
	return servertest.TranscodeFixture{New: func(config servertest.TranscodeConfig) http.Handler {
		return server.New(server.Config{MediaDir: config.MediaDir, CacheDir: config.CacheDir, FFmpeg: config.FFmpeg, FFprobe: config.FFprobe})
	}, WriteExecutable: writeExecutable, AssertSafari: assertSafariDefersMatroska, PlayableHLS: fakePlayableHLS(), PlayerScript: "/static/player.js?v=53"}
}

func TestUnsupportedContainerDefaultsToDirectPlaybackWithCompatibleFallback(t *testing.T) {
	transcodeFixture().UnsupportedContainerDefaultsToDirectPlaybackWithCompatibleFallback(t)
}

func TestUnsupportedCodecInMP4DefaultsToDirectPlaybackWithCompatibleFallback(t *testing.T) {
	transcodeFixture().UnsupportedCodecInMP4DefaultsToDirectPlaybackWithCompatibleFallback(t)
}

func TestViewerCanRequestSeekableCompatiblePlaylist(t *testing.T) {
	transcodeFixture().ViewerCanRequestSeekableCompatiblePlaylist(t)
}

func TestInterruptedCompatibleCacheIsRegeneratedAfterServerRestart(t *testing.T) {
	transcodeFixture().InterruptedCompatibleCacheIsRegeneratedAfterServerRestart(t)
}

func TestAdaptiveHLSPublishesACompleteTruthfulLadder(t *testing.T) {
	transcodeFixture().AdaptiveHLSPublishesACompleteTruthfulLadder(t, "4")
}

func TestAdaptiveHLSDoesNotAdvertiseOrEncodeAbsentAudio(t *testing.T) {
	transcodeFixture().AdaptiveHLSDoesNotAdvertiseOrEncodeAbsentAudio(t)
}

func readTestFile(t *testing.T, path string) string {
	t.Helper()
	return servertest.ReadTranscodeFile(t, path)
}
