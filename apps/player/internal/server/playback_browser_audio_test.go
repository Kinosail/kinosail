package server_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail-player/internal/server"
)

const chromeUserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36"

func TestAutomaticBrowserPlaybackNeverStartsWithSilentSurroundAudio(t *testing.T) {
	t.Parallel()
	media, data, tools := t.TempDir(), t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(media, "Zootopia 2 (2025).mkv"), []byte("media"), 0o600); err != nil {
		t.Fatal(err)
	}
	ffprobe := filepath.Join(tools, "ffprobe")
	probe := `{"streams":[` +
		`{"index":0,"codec_type":"video","codec_name":"hevc","profile":"Main 10","pix_fmt":"yuv420p10le","width":3840,"height":1608,"color_transfer":"bt709","disposition":{"default":1}},` +
		`{"index":1,"codec_type":"audio","codec_name":"eac3","channels":6,"tags":{"language":"eng"},"disposition":{"default":1}},` +
		`{"index":2,"codec_type":"audio","codec_name":"eac3","channels":8,"tags":{"language":"zho"},"disposition":{"default":0}},` +
		`{"index":3,"codec_type":"audio","codec_name":"ac3","channels":6,"tags":{"language":"yue"},"disposition":{"default":0}}],` +
		`"format":{"format_name":"matroska,webm","duration":"6458"}}`
	writeExecutable(t, ffprobe, "#!/bin/sh\nprintf '%s' '"+probe+"'\n")
	handler, id := firstWebItem(t, server.Config{Lifecycle: t.Context(), MediaDir: media, DataDir: data, FFprobe: ffprobe})

	for _, tc := range []struct {
		name, query, mode, videoCodec string
	}{
		{"browser without HEVC decoding", "", "transcode", "h264"},
		{"browser with HEVC decoding", "?videoCodecs=h264,hevc", "audio-transcode", "hevc"},
	} {
		request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/items/"+id+"/playback"+tc.query, nil)
		request.Header.Set("User-Agent", chromeUserAgent)
		var info struct {
			Plan struct {
				Mode, AudioCodec, VideoCodec string
				AudioCompatibilityRequired  bool
			}
		}
		mustJSON(t, serveRequest(handler, request), &info)
		if info.Plan.Mode != tc.mode || info.Plan.AudioCodec != "aac" || info.Plan.VideoCodec != tc.videoCodec || !info.Plan.AudioCompatibilityRequired {
			t.Fatalf("%s: automatic plan can start without audible sound: %+v", tc.name, info.Plan)
		}
	}

	watch := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/watch/"+id, nil)
	watch.Header.Set("User-Agent", chromeUserAgent)
	page := serveRequest(handler, watch).Body.String()
	if !strings.Contains(page, `data-audio-compatibility-required="true"`) || !strings.Contains(page, `data-hls="/hls/`) {
		t.Fatal("web player would try the original MKV with surround audio the browser cannot decode")
	}
}
