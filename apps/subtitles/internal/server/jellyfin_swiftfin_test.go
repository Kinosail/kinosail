package server_test

import (
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/MikeO7/kinosail-subtitles/internal/server"
)

func TestJellyfinPlaybackUsesSwiftfinMediaShape(t *testing.T) {
	t.Parallel()
	mediaDir, toolsDir := t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(mediaDir, "Film.mp4"), []byte("video"), 0o600); err != nil {
		t.Fatal(err)
	}
	ffprobe := filepath.Join(toolsDir, "ffprobe")
	writeExecutable(t, ffprobe, `#!/bin/sh
printf '%s' '{"streams":[{"index":0,"codec_type":"video","codec_name":"h264","level":40},{"index":1,"codec_type":"audio","codec_name":"dts","channels":1,"disposition":{"default":1}}],"format":{"format_name":"mov,mp4","duration":"60"}}'
`)
	handler := newJellyfinServer(t, server.Config{MediaDir: mediaDir, DataDir: t.TempDir(), FFprobe: ffprobe, RequireAuth: true})
	owner := signInTestProfile(t, handler, "/setup", "name=Owner&password=owner-password")
	token, _ := jellyfinLogin(t, handler, owner)
	items := jellyfinCall(t, handler, http.MethodGet, "/Items", "", token)
	var catalog jellyfinItems
	decodeJellyfin(t, items, &catalog)
	playback := jellyfinCall(t, handler, http.MethodPost, "/Items/"+catalog.Items[0].ID+"/PlaybackInfo", `{}`, token)
	var result struct {
		DefaultAudioStreamIndex int
		MediaSources            []struct {
			DefaultAudioStreamIndex int
			MediaStreams            []struct {
				Type  string
				Index int
				Level float64
			}
		}
	}
	decodeJellyfin(t, playback, &result)
	source := result.MediaSources[0]
	if playback.Code != http.StatusOK || source.DefaultAudioStreamIndex != 1 || source.MediaStreams[0].Level != 40 || source.MediaStreams[1].Type != "Audio" || source.MediaStreams[1].Index != 1 {
		t.Fatalf("playback = %d %q", playback.Code, playback.Body.String())
	}
}

func TestJellyfinPersonSearchReturnsEmptyCompatibleResult(t *testing.T) {
	t.Parallel()
	handler, token, _ := jellyfinTestServer(t)
	response := jellyfinCall(t, handler, http.MethodGet, "/Persons?searchTerm=beautiful&limit=20", "", token)
	var result struct {
		Items            []map[string]any
		TotalRecordCount int
	}
	decodeJellyfin(t, response, &result)
	if response.Code != http.StatusOK || result.Items == nil || len(result.Items) != 0 || result.TotalRecordCount != 0 {
		t.Fatalf("person search = %d %q", response.Code, response.Body.String())
	}
}
