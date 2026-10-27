package server_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/MikeO7/kinosail-player/internal/server"
)

func TestApplePlaybackKeepsChaptersInsideTheMovie(t *testing.T) {
	t.Parallel()
	media, tools := t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(media, "Film.mkv"), []byte("video"), 0o600); err != nil {
		t.Fatal(err)
	}
	ffprobe := filepath.Join(tools, "ffprobe")
	writeExecutable(t, ffprobe, "#!/bin/sh\nprintf '%s' '{\"streams\":[{\"codec_type\":\"video\",\"codec_name\":\"h264\",\"profile\":\"High\",\"width\":1920,\"height\":804},{\"codec_type\":\"audio\",\"codec_name\":\"truehd\",\"channels\":8,\"disposition\":{\"default\":1}}],\"chapters\":[{\"start_time\":\"0\",\"end_time\":\"7886.045\",\"tags\":{\"title\":\"Chapter 1\"}},{\"start_time\":\"7886.045\",\"end_time\":\"8430.112\",\"tags\":{\"title\":\"Chapter 2\"}}],\"format\":{\"format_name\":\"matroska,webm\",\"duration\":\"8430.089\"}}'\n")
	handler, id := firstWebItem(t, server.Config{MediaDir: media, FFprobe: ffprobe})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequestWithContext(t.Context(), http.MethodGet,
		"/api/v1/items/"+id+"/playback?videoCodecs=h264&audioCodecs=aac,mp3,ac3,eac3&hdrFormats=sdr&maxAudioChannels=8", nil))
	var playback struct {
		Duration   float64 `json:"duration"`
		Compatible string  `json:"compatible"`
		Media      struct {
			Duration float64 `json:"duration"`
		} `json:"media"`
		Chapters []struct {
			Start float64 `json:"start"`
			End   float64 `json:"end"`
		} `json:"chapters"`
	}
	if response.Code != http.StatusOK || json.Unmarshal(response.Body.Bytes(), &playback) != nil || playback.Compatible == "" || len(playback.Chapters) != 2 {
		t.Fatalf("playback = %d %q", response.Code, response.Body.String())
	}
	if playback.Duration != 8430.089 || playback.Media.Duration != 8430.089 {
		t.Fatalf("duration = %v %v", playback.Duration, playback.Media.Duration)
	}
	if last := playback.Chapters[1]; last.Start != 7886.045 || last.End != playback.Duration {
		t.Fatalf("last chapter = %+v, duration %v", last, playback.Duration)
	}
}
