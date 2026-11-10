package server_test

import (
	"archive/zip"
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/MikeO7/kinosail-subtitles/internal/server"
)

const styledArchiveScript = "[Script Info]\nScriptType: v4.00+\n\n[V4+ Styles]\nFormat: Name, Fontname\nStyle: Default,Arial\n\n[Events]\nFormat: Layer, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text\nDialogue: 0,0:00:01.00,0:00:02.50,Default,,0,0,0,,{\\an8}Hello there\\NGeneral\n"

func TestProviderArchiveWithOnlyStyledSubtitlesIsConvertedToSRT(t *testing.T) {
	for _, sample := range []struct {
		name   string
		files  map[string]string
		status int
		want   string
	}{
		{"one styled file", map[string]string{"Arrival.BluRay-GROUP.en.ass": styledArchiveScript}, http.StatusCreated, "1\n00:00:01,000 --> 00:00:02,500\nHello there\nGeneral\n\n"},
		{"text file wins", map[string]string{"Arrival.BluRay-GROUP.en.ssa": styledArchiveScript, "Arrival.BluRay-GROUP.en.srt": "1\n00:00:04,000 --> 00:00:05,000\nPlain text\n"}, http.StatusCreated, "1\n00:00:04,000 --> 00:00:05,000\nPlain text\n\n"},
		{"two styled files", map[string]string{"a.ass": styledArchiveScript, "b.ass": styledArchiveScript}, http.StatusBadGateway, ""},
		{"drawing", map[string]string{"a.ass": styledArchiveScript + "Dialogue: 0,0:00:03.00,0:00:04.00,Default,,0,0,0,,{\\p1}m 0 0 l 10 10{\\p0}\n"}, http.StatusBadGateway, ""},
		{"styled name without a script", map[string]string{"a.ass": "not a subtitle"}, http.StatusBadGateway, ""},
	} {
		t.Run(sample.name, func(t *testing.T) {
			provider := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				if request.URL.Path != "/arrival.zip" {
					fakeSubDL(writer, request)
					return
				}
				var archive bytes.Buffer
				files := zip.NewWriter(&archive)
				for name, content := range sample.files {
					file, _ := files.Create(name)
					_, _ = file.Write([]byte(content))
				}
				_ = files.Close()
				_, _ = writer.Write(archive.Bytes())
			}))
			t.Cleanup(provider.Close)
			media := t.TempDir()
			writeTestFile(t, filepath.Join(media, "Arrival.BluRay-GROUP.mp4"), "video")
			handler := server.New(server.Config{SubtitleApp: true, MediaDir: media, DataDir: t.TempDir(), CacheDir: t.TempDir(), Subtitles: server.SubtitleConfig{URL: provider.URL, APIKey: "key"}})
			id := firstSubtitleInventoryID(t, handler)
			result := requestJSON(t, handler, http.MethodPost, "/api/v1/subtitle-library/"+id+"/fetch", `{"language":"en"}`)
			if result.Code != sample.status {
				t.Fatalf("fetch = %d %s", result.Code, result.Body.String())
			}
			data, err := os.ReadFile(filepath.Join(media, "Arrival.BluRay-GROUP.en.srt"))
			if sample.want == "" {
				if files, listErr := os.ReadDir(media); listErr != nil || len(files) != 1 {
					t.Fatalf("rejected archive wrote files: %v %v", files, listErr)
				}
				return
			}
			if err != nil || string(data) != sample.want {
				t.Fatalf("sidecar = %q, %v; want %q", data, err, sample.want)
			}
		})
	}
}
