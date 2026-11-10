package server_test

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail-subtitles/internal/server"
)

func TestProviderSubtitleInTheWrongWritingSystemIsRejectedWithoutWriting(t *testing.T) {
	russian := "1\n00:00:01,000 --> 00:00:03,000\nПривет, как твои дела сегодня?\n\n2\n00:00:04,000 --> 00:00:06,000\nВсё хорошо, спасибо. Пойдём домой, Arrival.\n"
	english := "1\n00:00:01,000 --> 00:00:03,000\nHello, how are you doing today?\n\n2\n00:00:04,000 --> 00:00:06,000\nAll good, thank you. Café déjà vu.\n"
	for _, sample := range []struct {
		name, language, subtitle string
		status                   int
	}{
		{"cyrillic labelled english", "en", russian, http.StatusBadGateway},
		{"english labelled russian", "ru", english, http.StatusBadGateway},
		{"russian", "ru", russian, http.StatusCreated},
		{"english with accents", "en", english, http.StatusCreated},
	} {
		t.Run(sample.name, func(t *testing.T) {
			provider := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				if request.URL.Path == "/subtitles" {
					_ = json.NewEncoder(writer).Encode(map[string]any{
						"status": true, "results": []any{map[string]any{"name": "Arrival", "type": "movie"}},
						"subtitles": []any{map[string]any{"url": "http://" + request.Host + "/arrival.zip", "language": strings.ToUpper(sample.language), "release_name": "Arrival.BluRay-GROUP"}},
					})
					return
				}
				var archive bytes.Buffer
				files := zip.NewWriter(&archive)
				file, _ := files.Create("Arrival.BluRay-GROUP.srt")
				_, _ = file.Write([]byte(sample.subtitle))
				_ = files.Close()
				_, _ = writer.Write(archive.Bytes())
			}))
			t.Cleanup(provider.Close)
			media := t.TempDir()
			writeTestFile(t, filepath.Join(media, "Arrival.BluRay-GROUP.mp4"), "video")
			handler := server.New(server.Config{SubtitleApp: true, MediaDir: media, DataDir: t.TempDir(), CacheDir: t.TempDir(), Subtitles: server.SubtitleConfig{URL: provider.URL, APIKey: "key"}})
			id := firstSubtitleInventoryID(t, handler)
			result := requestJSON(t, handler, http.MethodPost, "/api/v1/subtitle-library/"+id+"/fetch", `{"language":"`+sample.language+`"}`)
			if result.Code != sample.status {
				t.Fatalf("fetch = %d %s", result.Code, result.Body.String())
			}
			files, err := os.ReadDir(media)
			if wanted := map[bool]int{true: 2, false: 1}[sample.status == http.StatusCreated]; err != nil || len(files) != wanted {
				t.Fatalf("media folder after fetch = %v, %v; want %d files", files, err, wanted)
			}
		})
	}
}
