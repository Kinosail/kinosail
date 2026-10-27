package server_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail-player/internal/server"
)

const stereoAppleQuery = "videoCodecs=h264,hevc&audioCodecs=aac,mp3,ac3,eac3&hdrFormats=sdr&maxAudioChannels=2"

func playbackAnswer(t *testing.T, name, probe, query string) (map[string]json.RawMessage, string) {
	t.Helper()
	media, tools := t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(media, name), []byte("video"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tools, "probe.json"), []byte(probe), 0o600); err != nil {
		t.Fatal(err)
	}
	ffprobe := filepath.Join(tools, "ffprobe")
	writeExecutable(t, ffprobe, "#!/bin/sh\ncase \" $* \" in *stream=index*) cat '"+filepath.Join(tools, "probe.json")+"' ;; *) printf '{\"frames\":[]}' ;; esac\n")
	handler, id := firstWebItem(t, server.Config{MediaDir: media, FFprobe: ffprobe})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/items/"+id+"/playback?"+query, nil))
	var answer map[string]json.RawMessage
	if response.Code != http.StatusOK || json.Unmarshal(response.Body.Bytes(), &answer) != nil {
		t.Fatalf("playback = %d %q", response.Code, response.Body.String())
	}
	return answer, response.Body.String()
}

func TestPlaybackKeepsMediaFactsForFilesWithManySubtitleTracks(t *testing.T) {
	t.Parallel()
	streams := []string{`{"index":0,"codec_type":"video","codec_name":"h264","profile":"High","width":1440,"height":1080}`, `{"index":1,"codec_type":"audio","codec_name":"aac","profile":"LC","channels":2}`}
	for index := 2; index < 71; index++ {
		streams = append(streams, fmt.Sprintf(`{"index":%d,"codec_type":"subtitle","codec_name":"subrip","tags":{"language":"eng"}}`, index))
	}
	answer, body := playbackAnswer(t, "Film.mkv", `{"streams":[`+strings.Join(streams, ",")+`],"format":{"format_name":"matroska,webm","duration":"1074.048"}}`, stereoAppleQuery)
	var media struct {
		Video     struct{ Codec string }
		Subtitles []json.RawMessage `json:"subtitles"`
	}
	if string(answer["duration"]) != "1074.048" || json.Unmarshal(answer["media"], &media) != nil || media.Video.Codec != "h264" || len(media.Subtitles) != 64 {
		t.Fatalf("playback lost media facts: %.600s", body)
	}
}

func TestPlaybackListsAreNeverNullWhenOnlyTheCompatibleStreamIsOffered(t *testing.T) {
	t.Parallel()
	answer, body := playbackAnswer(t, "Show S01E01.mkv", `{"streams":[{"index":0,"codec_type":"video","codec_name":"h264","profile":"High","width":1920,"height":1080},{"index":1,"codec_type":"audio","codec_name":"ac3","channels":6}],"chapters":[{"start_time":"0","end_time":"5","tags":{"title":"Cold open"}},{"start_time":"5","end_time":"65","tags":{"title":"Intro"}},{"start_time":"65","end_time":"1200","tags":{"title":"Episode"}}],"format":{"format_name":"matroska,webm","duration":"1200"}}`, stereoAppleQuery)
	if string(answer["directAllowed"]) != "false" || len(answer["compatible"]) == 0 {
		t.Fatalf("fixture must offer only the compatible stream: %.600s", body)
	}
	for _, key := range []string{"subtitles", "audio", "chapters", "markers", "autoSkip"} {
		if value := strings.TrimSpace(string(answer[key])); value == "" || value == "null" {
			t.Fatalf("%s = %q in %.600s", key, value, body)
		}
	}
}

func TestDirectPlaybackAlwaysNamesTheFileType(t *testing.T) {
	t.Parallel()
	for name, probe := range map[string]string{
		"Old Film (1999).avi":  `{"streams":[{"index":0,"codec_type":"video","codec_name":"mpeg4","width":640,"height":352},{"index":1,"codec_type":"audio","codec_name":"mp3","channels":2}],"format":{"format_name":"avi","duration":"303"}}`,
		"Home Film (2020).mov": `{"streams":[{"index":0,"codec_type":"video","codec_name":"hevc","profile":"Main","width":1920,"height":1036},{"index":1,"codec_type":"audio","codec_name":"eac3","channels":6}],"format":{"format_name":"mov,mp4,m4a,3gp,3g2,mj2","duration":"8963"}}`,
	} {
		answer, body := playbackAnswer(t, name, probe, "videoCodecs=h264,hevc&audioCodecs=aac,mp3,ac3,eac3&hdrFormats=sdr&maxAudioChannels=6")
		if string(answer["directAllowed"]) != "true" || len(answer["directType"]) < 3 || !strings.HasPrefix(string(answer["directType"]), `"video/`) {
			t.Fatalf("%s directType = %s in %.600s", name, answer["directType"], body)
		}
	}
}

func TestPlaybackAlwaysIncludesTheDuration(t *testing.T) {
	t.Parallel()
	answer, body := playbackAnswer(t, "Film.mkv", `{"streams":[{"index":0,"codec_type":"video","codec_name":"h264","width":640,"height":360},{"index":1,"codec_type":"audio","codec_name":"aac","channels":2}],"format":{"format_name":"matroska,webm"}}`, stereoAppleQuery)
	if string(answer["duration"]) != "0" {
		t.Fatalf("duration = %q in %.600s", answer["duration"], body)
	}
}

func TestDolbyVisionWithAnHDR10BaseLayerHasACompatibleStream(t *testing.T) {
	t.Parallel()
	answer, body := playbackAnswer(t, "Film (2021).mov", `{"streams":[{"index":0,"codec_type":"video","codec_name":"hevc","profile":"Main 10","pix_fmt":"yuv420p10le","width":1920,"height":1036,"color_space":"bt2020nc","color_transfer":"smpte2084","color_primaries":"bt2020","side_data_list":[{"side_data_type":"DOVI configuration record","dv_profile":8,"dv_bl_signal_compatibility_id":0}]},{"index":1,"codec_type":"audio","codec_name":"eac3","channels":6}],"format":{"format_name":"mov,mp4,m4a,3gp,3g2,mj2","duration":"8962.5"}}`, stereoAppleQuery)
	var plan struct {
		Allowed   bool   `json:"allowed"`
		Mode      string `json:"mode"`
		ColorMode string `json:"colorMode"`
	}
	if len(answer["compatible"]) == 0 || json.Unmarshal(answer["compatiblePlan"], &plan) != nil || !plan.Allowed || plan.Mode != "transcode" || plan.ColorMode != "tone-map-sdr" {
		t.Fatalf("Dolby Vision fallback = %s in %.900s", answer["compatiblePlan"], body)
	}
}
