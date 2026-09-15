package servertest

import (
	"net/http"
	"os"
	"path/filepath"
	"testing"
)

// PlaybackAPIConfig describes the app configuration used by playback API contracts.
type PlaybackAPIConfig struct {
	MediaDir, DataDir, FFprobe, FFmpeg, HardwareOS, HardwareArch string
	RequireAuth, ProbeHardware                                   bool
	HardwareDevices                                              []string
}

// PlaybackAPIFixture binds playback requests to the actual app handler.
type PlaybackAPIFixture struct {
	New    func(PlaybackAPIConfig) http.Handler
	SignIn func(*testing.T, http.Handler, string, string) *http.Cookie
}

// PlaybackAPIExposesSelectableAudioSources checks the real app's playback API response.
func (fixture PlaybackAPIFixture) PlaybackAPIExposesSelectableAudioSources(t *testing.T) {
	t.Helper()
	t.Parallel()
	media, data, tools := t.TempDir(), t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(media, "Film.mkv"), []byte("video"), 0o600); err != nil {
		t.Fatal(err)
	}
	ffprobe := filepath.Join(tools, "ffprobe")
	WriteExecutable(t, ffprobe, "#!/bin/sh\nprintf '%s' '{\"streams\":[{\"codec_type\":\"video\",\"codec_name\":\"h264\"},{\"codec_type\":\"audio\",\"codec_name\":\"aac\",\"tags\":{\"language\":\"eng\"}},{\"codec_type\":\"audio\",\"codec_name\":\"ac3\",\"tags\":{\"language\":\"spa\"}}]}'\n")
	handler := fixture.New(PlaybackAPIConfig{MediaDir: media, DataDir: data, FFprobe: ffprobe, RequireAuth: true})
	owner := fixture.SignIn(t, handler, "/setup", "name=Owner&password=owner-password")
	var session struct {
		Token string `json:"token"`
	}
	session.Token = owner.Value
	var catalog struct {
		Items []struct {
			ID string `json:"id"`
		} `json:"items"`
	}
	MustJSON(t, APICall(t, handler, session.Token, http.MethodGet, "/api/v1/library", nil), &catalog)
	AssertAPIBody(t, APICall(t, handler, session.Token, http.MethodGet, "/api/v1/items/"+catalog.Items[0].ID+"/playback", nil), http.StatusOK, `"index":0,"label":"ENG · AAC","source":"/hls/`, `"index":1,"label":"SPA · AC3","source":"/hls/`+catalog.Items[0].ID+`/audio/1/index.m3u8"`)
}

// PlaybackAPIUsesTheSmallestClientSupportedTransformation checks the real app's playback API response.
func (fixture PlaybackAPIFixture) PlaybackAPIUsesTheSmallestClientSupportedTransformation(t *testing.T) {
	t.Helper()
	t.Parallel()
	media, tools := t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(media, "Film.mkv"), []byte("video"), 0o600); err != nil {
		t.Fatal(err)
	}
	ffprobe := filepath.Join(tools, "ffprobe")
	WriteExecutable(t, ffprobe, "#!/bin/sh\nprintf '%s' '{\"streams\":[{\"codec_type\":\"video\",\"codec_name\":\"hevc\"},{\"codec_type\":\"audio\",\"codec_name\":\"aac\"}],\"format\":{\"format_name\":\"matroska\"}}'\n")
	device := filepath.Join(tools, "renderD128")
	if err := os.WriteFile(device, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	ffmpeg := filepath.Join(tools, "ffmpeg")
	WriteExecutable(t, ffmpeg, "#!/bin/sh\n"+HardwareSmokeHLS()+"printf 'libx264 libx265 libsvtav1 libvpx-vp9 h264_qsv hevc_qsv av1_qsv vp9_qsv qsv\\n'\n")
	handler, id := FirstWebItem(t, fixture.New(PlaybackAPIConfig{MediaDir: media, FFprobe: ffprobe, FFmpeg: ffmpeg, ProbeHardware: true, HardwareDevices: []string{device}, HardwareOS: "linux", HardwareArch: "amd64"}))
	remux := APICall(t, handler, "", http.MethodGet, "/api/v1/items/"+id+"/playback?videoCodecs=av1%2Chevc%2Ch264", nil)
	AssertAPIBody(t, remux, http.StatusOK, `"compatible":"/hls/`+id+`/p/r-a0-s0-none-t0-b0/index.m3u8"`)
	transcode := APICall(t, handler, "", http.MethodGet, "/api/v1/items/"+id+"/playback?videoCodecs=av1%2Ch264", nil)
	AssertAPIBody(t, transcode, http.StatusOK, `"compatible":"/hls/`+id+`/p/t-a0-s0-none-t0-b0/index.m3u8"`)
}

// AssertProbe checks playback output for controlled media evidence.
func (fixture PlaybackAPIFixture) AssertProbe(t *testing.T, probe string, expected ...string) {
	t.Helper()
	media, tools := t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(media, "Film.mkv"), []byte("video"), 0o600); err != nil {
		t.Fatal(err)
	}
	ffprobe := filepath.Join(tools, "ffprobe")
	WriteExecutable(t, ffprobe, "#!/bin/sh\nprintf '%s' '"+probe+"'\n")
	handler, id := FirstWebItem(t, fixture.New(PlaybackAPIConfig{MediaDir: media, FFprobe: ffprobe}))
	response := APICall(t, handler, "", http.MethodGet, "/api/v1/items/"+id+"/playback", nil)
	AssertAPIBody(t, response, http.StatusOK, expected...)
}
