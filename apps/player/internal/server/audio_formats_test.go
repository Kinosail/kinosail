package server_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/MikeO7/kinosail-player/internal/server"
	"github.com/MikeO7/kinosail/packages/playback"
)

// Exercise actual encoders, HTTP playlists and decoded samples, not file extensions alone.
func TestAudioFormatsDecodeThroughCompatiblePlayback(t *testing.T) {
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("FFmpeg is required for the audio format integration test")
	}
	ffprobe, err := exec.LookPath("ffprobe")
	if err != nil {
		t.Skip("FFprobe is required for the audio format integration test")
	}
	for _, sample := range []struct{ name, codec string }{
		{"Song.wav", "pcm_s24le"},
		{"Song.flac", "flac"},
		{"Song.ogg", "vorbis"},
		{"Song.opus", "libopus"},
		{"Song.wma", "wmav2"},
		{"Song.m4a", "alac"},
		{"Book.m4b", "aac"},
		{"Song.mp3", "libmp3lame"},
		{"Song.aifc", "pcm_s16le"},
		{"Song.caf", "pcm_s24le"},
		{"Song.wv", "wavpack"},
		{"Song.tta", "tta"},
		{"Song.ac3", "ac3"},
		{"Song.eac3", "eac3"},
	} {
		t.Run(sample.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
			defer cancel()
			media, cache := t.TempDir(), t.TempDir()
			command := exec.CommandContext(ctx, ffmpeg, "-hide_banner", "-loglevel", "error", "-f", "lavfi", "-i", "sine=frequency=440:duration=3", "-ac", "2", "-c:a", sample.codec, "-strict", "experimental", filepath.Join(media, sample.name)) //nolint:gosec // G204: tool paths come from LookPath; all arguments are fixed synthetic fixtures or httptest URLs.
			if output, err := command.CombinedOutput(); err != nil {
				t.Fatalf("generate %s: %v: %s", sample.name, err, output)
			}
			handler, id := formatTestItem(t, server.Config{Lifecycle: ctx, MediaDir: media, CacheDir: cache, FFmpeg: ffmpeg, FFprobe: ffprobe})
			response := apiCall(t, handler, "", http.MethodGet, "/api/v1/items/"+id+"/playback?videoCodecs=h264&audioCodecs=aac,mp3&audioChannels=2", nil)
			var info struct {
				Direct, Compatible   string
				Plan, CompatiblePlan playback.PlaybackPlan
			}
			assertAPIBody(t, response, http.StatusOK)
			mustJSON(t, response, &info)
			if info.Direct == "" || info.Compatible == "" || info.Plan.Mode != "direct" || info.CompatiblePlan.Mode != "audio-transcode" {
				t.Fatalf("playback contract: %d %s", response.Code, response.Body.String())
			}
			assertAudioContentType(t, handler, info.Direct, sample.name)
			assertCompatibleAudioSamples(t, ctx, handler, info.Compatible, ffmpeg, ffprobe)
		})
	}
}

func TestAudioHLSRejectsInvalidRecipesBeforeEncoding(t *testing.T) {
	media, cache, tools := t.TempDir(), t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(media, "Song.flac"), []byte("audio"), 0o600); err != nil {
		t.Fatal(err)
	}
	ffprobe, ffmpeg := filepath.Join(tools, "ffprobe"), filepath.Join(tools, "ffmpeg")
	writeExecutable(t, ffprobe, "#!/bin/sh\nprintf '%s' '{\"streams\":[{\"codec_type\":\"audio\",\"codec_name\":\"flac\"}],\"format\":{\"duration\":\"30\"}}'\n")
	marker := filepath.Join(tools, "encoded")
	writeExecutable(t, ffmpeg, "#!/bin/sh\ntouch '"+marker+"'\n")
	handler, id := formatTestItem(t, server.Config{Lifecycle: t.Context(), MediaDir: media, CacheDir: cache, FFmpeg: ffmpeg, FFprobe: ffprobe})
	// Populate source facts before comparing output directories after rejected requests.
	apiCall(t, handler, "", http.MethodGet, "/api/v1/items/"+id+"/playback", nil)
	before, err := os.ReadDir(cache)
	if err != nil {
		t.Fatal(err)
	}
	for _, token := range []string{"", "unknown", strings.Repeat("a", 8193), "a-a32-s0-none-t0-b0", "a-a1-s0-none-t0-b0", "r-a0-s0-none-t0-b0", "t-a0-s0-none-t0-b0", "a-a0-s1-text-t0-b0", "a-a0-s0-none-t1-b0", "a-a0-s0-none-t0-b0-z640x360", "a-a0-s0-none-t0-b0-o31000"} {
		response := apiCall(t, handler, "", http.MethodGet, "/hls/"+id+"/p/"+token+"/index.m3u8", nil)
		if response.Code < 300 {
			t.Fatalf("invalid audio recipe returned %d", response.Code)
		}
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("invalid request started encoder: %v", err)
	}
	entries, err := os.ReadDir(cache)
	if err != nil || len(entries) != len(before) {
		t.Fatalf("invalid request changed cache: %v, %v", entries, err)
	}
	for index, entry := range entries {
		if entry.Name() != before[index].Name() {
			t.Fatal("invalid request created HLS output")
		}
	}
}

func TestVideoFallbackStillDecodesWithAudioCompatibilityEnabled(t *testing.T) {
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("FFmpeg is required for the video regression test")
	}
	ffprobe, err := exec.LookPath("ffprobe")
	if err != nil {
		t.Skip("FFprobe is required for the video regression test")
	}
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	media := t.TempDir()
	command := exec.CommandContext(ctx, ffmpeg, "-hide_banner", "-loglevel", "error", "-f", "lavfi", "-i", "testsrc2=size=320x180:rate=24:duration=3", "-f", "lavfi", "-i", "sine=duration=3", "-c:v", "ffv1", "-c:a", "pcm_s16le", filepath.Join(media, "Movie.mkv")) //nolint:gosec // G204: tool paths come from LookPath; all arguments are fixed synthetic fixtures or httptest URLs.
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("generate video: %v: %s", err, output)
	}
	handler, id := formatTestItem(t, server.Config{Lifecycle: ctx, MediaDir: media, CacheDir: t.TempDir(), FFmpeg: ffmpeg, FFprobe: ffprobe})
	response := apiCall(t, handler, "", http.MethodGet, "/api/v1/items/"+id+"/playback", nil)
	var info struct {
		Direct, Compatible string
		CompatiblePlan     playback.PlaybackPlan
	}
	if response.Code != http.StatusOK || json.Unmarshal(response.Body.Bytes(), &info) != nil || info.Direct == "" || info.Compatible == "" || info.CompatiblePlan.Mode != "transcode" {
		t.Fatalf("video sources: %d %s", response.Code, response.Body.String())
	}
	host := httptest.NewServer(handler)
	defer host.Close()
	command = exec.CommandContext(ctx, ffmpeg, "-hide_banner", "-loglevel", "error", "-i", host.URL+info.Compatible, "-map", "0:v:0", "-map", "0:a:0", "-t", "2", "-f", "null", "-") //nolint:gosec // G204: tool paths come from LookPath; all arguments are fixed synthetic fixtures or httptest URLs.
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("decode compatible video and audio: %v: %s", err, output)
	}
}

func formatTestItem(t *testing.T, config server.Config) (http.Handler, string) {
	t.Helper()
	handler := server.New(config)
	response := apiCall(t, handler, "", http.MethodGet, "/api/v1/library", nil)
	var library struct{ Items []struct{ ID string } }
	if response.Code != http.StatusOK || json.Unmarshal(response.Body.Bytes(), &library) != nil || len(library.Items) != 1 {
		t.Fatalf("fixture library: %d %s", response.Code, response.Body.String())
	}
	return handler, library.Items[0].ID
}

func assertCompatibleAudioSamples(t *testing.T, ctx context.Context, handler http.Handler, compatible, ffmpeg, ffprobe string) {
	t.Helper()
	master := apiCall(t, handler, "", http.MethodGet, compatible, nil)
	assertAPIBody(t, master, http.StatusOK, `CODECS="mp4a.40.2"`, "audio/index.m3u8")
	if strings.Contains(master.Body.String(), "RESOLUTION") {
		t.Fatal("audio master declared a video resolution")
	}
	host := httptest.NewServer(handler)
	defer host.Close()
	command := exec.CommandContext(ctx, ffprobe, "-v", "error", "-show_entries", "stream=codec_name,codec_type", "-of", "json", host.URL+compatible) //nolint:gosec // G204: tool paths come from LookPath; all arguments are fixed synthetic fixtures or httptest URLs.
	output, err := command.CombinedOutput()
	var observed struct {
		Streams []struct {
			CodecName string `json:"codec_name"`
			CodecType string `json:"codec_type"`
		}
	}
	if err != nil || json.Unmarshal(output, &observed) != nil || len(observed.Streams) != 1 || observed.Streams[0].CodecName != "aac" || observed.Streams[0].CodecType != "audio" {
		t.Fatalf("probe compatible output: %v: %s", err, output)
	}
	command = exec.CommandContext(ctx, ffmpeg, "-hide_banner", "-loglevel", "error", "-i", host.URL+compatible, "-map", "0:a:0", "-t", "2", "-f", "s16le", "-") //nolint:gosec // G204: tool paths come from LookPath; all arguments are fixed synthetic fixtures or httptest URLs.
	output, err = command.Output()
	if err != nil || len(output) < 44100*2*2 {
		t.Fatalf("decoded PCM: %d bytes, %v", len(output), err)
	}
}

func assertAudioContentType(t *testing.T, handler http.Handler, directURL, name string) {
	t.Helper()
	if expected := map[string]string{".m4a": "audio/mp4", ".m4b": "audio/mp4", ".flac": "audio/flac"}[filepath.Ext(name)]; expected != "" {
		direct := apiCall(t, handler, "", http.MethodGet, directURL, nil)
		if direct.Code != http.StatusOK || direct.Header().Get("Content-Type") != expected {
			t.Fatalf("direct audio content type: %d %q", direct.Code, direct.Header().Get("Content-Type"))
		}
	}
}
