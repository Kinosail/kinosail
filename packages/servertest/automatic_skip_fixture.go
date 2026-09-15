package servertest

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"
)

type AutomaticSkipConfig struct {
	Lifecycle                                    context.Context
	MediaDir, DataDir, CacheDir, FFprobe, FFmpeg string
	Jellyfin                                     bool
}
type AutomaticSkipFixture struct {
	New                   func(*testing.T, AutomaticSkipConfig) http.Handler
	TOTP                  func(*testing.T, string, time.Time) string
	UnescapeSubtitleQuery bool
	CopiedHLSSegments     int
}

// NewAutomaticSkipFixture keeps both app factories on the same fixture policy.
func NewAutomaticSkipFixture[Config any](configure func(AutomaticSkipConfig) Config, plain func(Config) http.Handler, jellyfin func(*testing.T, Config) http.Handler, totp func(*testing.T, string, time.Time) string, unescape bool, segments int) AutomaticSkipFixture {
	return AutomaticSkipFixture{
		New: func(t *testing.T, fixture AutomaticSkipConfig) http.Handler {
			config := configure(fixture)
			if fixture.Jellyfin {
				return jellyfin(t, config)
			}
			return plain(config)
		},
		TOTP: totp, UnescapeSubtitleQuery: unescape, CopiedHLSSegments: segments,
	}
}

func (fixture AutomaticSkipFixture) server(t *testing.T) (http.Handler, string, string, string, string, string) {
	return fixture.serverWithFrames(t, `[{"key_frame":1,"best_effort_timestamp_time":"0"},{"key_frame":1,"best_effort_timestamp_time":"10"},{"key_frame":1,"best_effort_timestamp_time":"20"},{"key_frame":1,"best_effort_timestamp_time":"60"}]`)
}

func (fixture AutomaticSkipFixture) serverWithFrames(t *testing.T, frames string) (http.Handler, string, string, string, string, string) {
	t.Helper()
	media, data, cache, tools := t.TempDir(), t.TempDir(), t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(media, "Episode.S01E01.mp4"), []byte("video"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(media, "Episode.S01E01.en.srt"), []byte("1\n00:00:12,000 --> 00:00:14,000\nInside intro\n\n2\n00:00:22,000 --> 00:00:24,000\nAfter intro\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	ffprobe := filepath.Join(tools, "ffprobe")
	WriteExecutable(t, ffprobe, fmt.Sprintf(`#!/bin/sh
case " $* " in
  *" -show_frames "*) printf '%%s' '{"frames":%s}' ;;
  *) printf '%%s' '{"streams":[{"index":0,"codec_type":"video","codec_name":"h264","width":640,"height":360},{"index":1,"codec_type":"audio","codec_name":"aac","disposition":{"default":1}}],"chapters":[{"start_time":"10","end_time":"20","tags":{"title":"Intro"}}],"format":{"format_name":"mp4","duration":"60"}}' ;;
esac
`, frames))
	arguments, ffmpeg := filepath.Join(tools, "arguments"), filepath.Join(tools, "ffmpeg")
	WriteExecutable(t, ffmpeg, fmt.Sprintf("#!/bin/sh\nprintf '%%s\\n' \"$*\" >> '%s'\n", arguments)+PlayableHLS())
	handler := fixture.New(t, AutomaticSkipConfig{MediaDir: media, DataDir: data, CacheDir: cache, FFprobe: ffprobe, FFmpeg: ffmpeg, Jellyfin: true})
	setup := APICall(t, handler, "", http.MethodPost, "/api/v1/setup", map[string]any{"name": "Owner", "password": "owner-password", "device": "API test", "totp": true})
	var apiSession struct {
		Token string `json:"token"`
		TOTP  struct {
			Secret string `json:"secret"`
		} `json:"totp"`
	}
	MustJSON(t, setup, &apiSession)
	AssertAPIBody(t, APICall(t, handler, apiSession.Token, http.MethodPut, "/api/v1/me/mfa", map[string]any{"code": fixture.TOTP(t, apiSession.TOTP.Secret, time.Now())}), http.StatusOK, `"enabled":true`)
	var catalog struct {
		Items []struct {
			ID string `json:"id"`
		} `json:"items"`
	}
	MustJSON(t, APICall(t, handler, apiSession.Token, http.MethodGet, "/api/v1/library", nil), &catalog)
	items := automaticSkipLibrary(t, handler, apiSession.Token)
	var library struct{ Items []struct{ ID string } }
	decodeAutomaticSkipJellyfin(t, items, &library)
	return handler, apiSession.Token, apiSession.Token, catalog.Items[0].ID, library.Items[0].ID, arguments
}
