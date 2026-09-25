package server_test

import (
	"context"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/MikeO7/kinosail-player/internal/server"
	"github.com/MikeO7/kinosail/packages/playback"
)

func TestPlaybackAudioEnhancementsUseAnEffectSpecificCompatibleStream(t *testing.T) {
	media, tools := t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(media, "Song.m4a"), []byte("audio"), 0o600); err != nil {
		t.Fatal(err)
	}
	ffprobe := filepath.Join(tools, "ffprobe")
	writeExecutable(t, ffprobe, `#!/bin/sh
printf '%s' '{"streams":[{"index":0,"codec_type":"audio","codec_name":"aac","channels":2,"disposition":{"default":1}}],"format":{"duration":"30","format_name":"mov,mp4,m4a,3gp,3g2,mj2"}}'
`)
	config := server.Config{MediaDir: media, DataDir: t.TempDir(), CacheDir: t.TempDir(), FFprobe: ffprobe, FFmpeg: "/bin/false"}
	handler, id := formatTestItem(t, config)
	var preferences map[string]any
	mustJSON(t, apiCall(t, handler, "", http.MethodGet, "/api/v1/me/media-preferences", nil), &preferences)
	defaults := preferences["playback"].(map[string]any)
	defaults["dialogueBoost"], defaults["nightMode"] = true, true
	assertAPIBody(t, apiCall(t, handler, "", http.MethodPut, "/api/v1/me/media-preferences", preferences), http.StatusOK)

	var result struct {
		Direct, Compatible string
		CompatiblePlan     playback.PlaybackPlan
		CompatibleLabel    string
	}
	response := apiCall(t, handler, "", http.MethodGet, "/api/v1/items/"+id+"/playback?audioCodecs=aac", nil)
	mustJSON(t, response, &result)
	if response.Code != http.StatusOK || result.Direct != "" || result.CompatiblePlan.Mode != "audio-transcode" || result.CompatibleLabel != "Enhancing audio" || !strings.Contains(result.Compatible, "-e3/") {
		t.Fatalf("enhanced playback = %d %s", response.Code, response.Body.String())
	}

	defaults["nightMode"] = false
	assertAPIBody(t, apiCall(t, handler, "", http.MethodPut, "/api/v1/items/"+id+"/playback-preferences", defaults), http.StatusOK)
	response = apiCall(t, handler, "", http.MethodGet, "/api/v1/items/"+id+"/playback?audioCodecs=aac", nil)
	result = struct {
		Direct, Compatible string
		CompatiblePlan     playback.PlaybackPlan
		CompatibleLabel    string
	}{}
	mustJSON(t, response, &result)
	if result.Direct != "" || result.CompatibleLabel != "Boosting dialog" || !strings.Contains(result.Compatible, "-e1/") {
		t.Fatalf("title enhancement override = %d %s", response.Code, response.Body.String())
	}
	restarted, restoredID := formatTestItem(t, config)
	if restoredID != id {
		t.Fatalf("item changed after restart: %q to %q", id, restoredID)
	}
	var restored struct {
		Playback   struct{ DialogueBoost, NightMode bool }
		Overridden bool
	}
	mustJSON(t, apiCall(t, restarted, "", http.MethodGet, "/api/v1/items/"+id+"/playback-preferences", nil), &restored)
	if !restored.Overridden || !restored.Playback.DialogueBoost || restored.Playback.NightMode {
		t.Fatalf("saved title enhancement lost after restart: %+v", restored)
	}
	var profile struct{ Playback struct{ DialogueBoost, NightMode bool } }
	mustJSON(t, apiCall(t, restarted, "", http.MethodGet, "/api/v1/me/media-preferences", nil), &profile)
	if !profile.Playback.DialogueBoost || !profile.Playback.NightMode {
		t.Fatalf("saved profile enhancements lost after restart: %+v", profile)
	}
	response = apiCall(t, restarted, "", http.MethodGet, "/api/v1/items/"+id+"/playback?audioCodecs=aac", nil)
	result = struct {
		Direct, Compatible string
		CompatiblePlan     playback.PlaybackPlan
		CompatibleLabel    string
	}{}
	mustJSON(t, response, &result)
	if response.Code != http.StatusOK || result.CompatibleLabel != "Boosting dialog" || !strings.Contains(result.Compatible, "-e1/") {
		t.Fatalf("restored enhancement stream = %d %s", response.Code, response.Body.String())
	}
}

func TestPlaybackAudioEnhancementsRespectViewerTranscodePermission(t *testing.T) {
	media, data, cache, tools := t.TempDir(), t.TempDir(), t.TempDir(), t.TempDir()
	if err := os.Mkdir(filepath.Join(media, "Movies"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(media, "Movies", "Song.flac"), []byte("audio"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(data, "settings.json"), []byte(`{"name":"Kinosail","libraries":["Movies"]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	ffprobe := filepath.Join(tools, "ffprobe")
	writeExecutable(t, ffprobe, `#!/bin/sh
printf '%s' '{"streams":[{"index":0,"codec_type":"audio","codec_name":"flac","channels":2,"disposition":{"default":1}}],"format":{"duration":"30"}}'
`)
	handler := server.New(server.Config{MediaDir: media, DataDir: data, CacheDir: cache, FFprobe: ffprobe, FFmpeg: "/bin/false", RequireAuth: true})
	owner := signInTestProfile(t, handler, "/setup", "name=Owner&password=owner-password")
	var catalog struct{ Items []struct{ ID string } }
	mustJSON(t, apiCall(t, handler, owner.Value, http.MethodGet, "/api/v1/library", nil), &catalog)
	if len(catalog.Items) != 1 {
		t.Fatalf("fixture library = %+v", catalog.Items)
	}
	assertAPIBody(t, apiCall(t, handler, owner.Value, http.MethodPost, "/api/v1/profiles", map[string]any{
		"name": "Listener", "password": "viewer-password", "rating": "all", "libraries": []string{"Movies"},
	}), http.StatusCreated)
	var session struct {
		Token string `json:"token"`
	}
	mustJSON(t, apiCall(t, handler, "", http.MethodPost, "/api/v1/session", map[string]string{"name": "Listener", "password": "viewer-password"}), &session)
	enrollTestAPIFactor(t, handler, session.Token)
	var preferences map[string]any
	mustJSON(t, apiCall(t, handler, session.Token, http.MethodGet, "/api/v1/me/media-preferences", nil), &preferences)
	preferences["playback"].(map[string]any)["dialogueBoost"] = true
	assertAPIBody(t, apiCall(t, handler, session.Token, http.MethodPut, "/api/v1/me/media-preferences", preferences), http.StatusOK)

	before, err := os.ReadDir(cache)
	if err != nil {
		t.Fatal(err)
	}
	response := apiCall(t, handler, session.Token, http.MethodGet, "/api/v1/items/"+catalog.Items[0].ID+"/playback", nil)
	after, err := os.ReadDir(cache)
	if err != nil {
		t.Fatal(err)
	}
	if response.Code != http.StatusForbidden || !strings.Contains(response.Body.String(), "audio enhancements require transcoding permission") || len(after) != len(before) {
		t.Fatalf("denied enhancement = %d %q, cache entries %d -> %d", response.Code, response.Body.String(), len(before), len(after))
	}
}

func TestPlaybackAudioEnhancementStreamDecodes(t *testing.T) {
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("FFmpeg is required for the audio enhancement integration test")
	}
	ffprobe, err := exec.LookPath("ffprobe")
	if err != nil {
		t.Skip("FFprobe is required for the audio enhancement integration test")
	}
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	media := t.TempDir()
	command := exec.CommandContext(ctx, ffmpeg, "-hide_banner", "-loglevel", "error", "-f", "lavfi", "-i", "sine=frequency=1200:duration=3", "-ac", "2", filepath.Join(media, "Dialog.wav")) //nolint:gosec // G204: the tool path and synthetic fixture are test-controlled.
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("generate audio: %v: %s", err, output)
	}
	handler, id := formatTestItem(t, server.Config{Lifecycle: ctx, MediaDir: media, CacheDir: t.TempDir(), FFmpeg: ffmpeg, FFprobe: ffprobe})
	var preferences map[string]any
	mustJSON(t, apiCall(t, handler, "", http.MethodGet, "/api/v1/me/media-preferences", nil), &preferences)
	playback := preferences["playback"].(map[string]any)
	playback["dialogueBoost"], playback["nightMode"] = true, true
	assertAPIBody(t, apiCall(t, handler, "", http.MethodPut, "/api/v1/me/media-preferences", preferences), http.StatusOK)
	var result struct{ Compatible string }
	mustJSON(t, apiCall(t, handler, "", http.MethodGet, "/api/v1/items/"+id+"/playback?audioCodecs=aac", nil), &result)
	assertCompatibleAudioSamples(t, ctx, handler, result.Compatible, ffmpeg, ffprobe)
}
