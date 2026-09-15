package server_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail-subtitles/internal/server"
)

func TestMusicPlaybackExposesReplayGainMetadataThroughAPIAndWeb(t *testing.T) {
	t.Parallel()

	media, tools := t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(media, "Song.flac"), []byte("audio"), 0o600); err != nil {
		t.Fatal(err)
	}
	ffprobe := filepath.Join(tools, "ffprobe")
	writeExecutable(t, ffprobe, `#!/bin/sh
printf '%s' '{"streams":[{"codec_type":"audio","codec_name":"flac"}],"format":{"duration":"180","tags":{"title":"Song","artist":"Artist","replaygain_track_gain":"-7.23 dB","replaygain_album_gain":"-8.10 dB"}}}'
`)
	handler := newMusicHandler(t, media, ffprobe)
	listing := apiCall(t, handler, "", http.MethodGet, "/api/v1/library?view=music", nil)
	var catalog struct {
		Items []struct {
			ID string `json:"id"`
		} `json:"items"`
	}
	mustJSON(t, listing, &catalog)
	if len(catalog.Items) != 1 {
		t.Fatalf("music listing = %q", listing.Body.String())
	}
	id := catalog.Items[0].ID

	playback := apiCall(t, handler, "", http.MethodGet, "/api/v1/items/"+id+"/playback", nil)
	var result struct {
		ReplayGain struct {
			TrackDB float64 `json:"trackDb"`
			AlbumDB float64 `json:"albumDb"`
		} `json:"replayGain"`
	}
	mustJSON(t, playback, &result)
	if playback.Code != http.StatusOK || result.ReplayGain.TrackDB != -7.23 || result.ReplayGain.AlbumDB != -8.1 {
		t.Fatalf("playback = %d %q", playback.Code, playback.Body.String())
	}

	page := httptest.NewRecorder()
	handler.ServeHTTP(page, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/watch/"+id, nil))
	if page.Code != http.StatusOK || !strings.Contains(page.Body.String(), `data-replaygain-track="-7.23"`) || !strings.Contains(page.Body.String(), `data-replaygain-album="-8.1"`) {
		t.Fatalf("web player = %d %q", page.Code, page.Body.String())
	}
}

func TestMusicPlaybackUsesR128WhenReplayGainIsInvalid(t *testing.T) {
	t.Parallel()

	media, tools := t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(media, "Song.flac"), []byte("audio"), 0o600); err != nil {
		t.Fatal(err)
	}
	ffprobe := filepath.Join(tools, "ffprobe")
	writeExecutable(t, ffprobe, `#!/bin/sh
printf '%s' '{"streams":[{"codec_type":"audio","codec_name":"flac"}],"format":{"duration":"180","tags":{"replaygain_track_gain":"not-a-gain","r128_track_gain":"-1024","replaygain_album_gain":"500 dB","r128_album_gain":"nan"}}}'
`)
	handler := newMusicHandler(t, media, ffprobe)
	listing := apiCall(t, handler, "", http.MethodGet, "/api/v1/library?view=music", nil)
	var catalog struct {
		Items []struct {
			ID string `json:"id"`
		} `json:"items"`
	}
	mustJSON(t, listing, &catalog)
	if len(catalog.Items) != 1 {
		t.Fatalf("music listing = %q", listing.Body.String())
	}

	playback := apiCall(t, handler, "", http.MethodGet, "/api/v1/items/"+catalog.Items[0].ID+"/playback", nil)
	body := playback.Body.String()
	if playback.Code != http.StatusOK || !strings.Contains(body, `"trackDb":-4`) || strings.Contains(body, `"albumDb"`) {
		t.Fatalf("invalid gain handling = %d %q", playback.Code, body)
	}
}

func newMusicHandler(t *testing.T, media, ffprobe string) http.Handler {
	t.Helper()
	return server.New(server.Config{MediaDir: media, FFprobe: ffprobe})
}
