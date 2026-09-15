package server_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail-subtitles/internal/server"
)

func subtitleOverviewServer(t *testing.T, files int, covered bool) http.Handler {
	t.Helper()
	media := t.TempDir()
	for i := 1; i <= files; i++ {
		writeTestFile(t, filepath.Join(media, fmt.Sprintf("Film %d.mp4", i)), "video")
		if covered {
			writeTestFile(t, filepath.Join(media, fmt.Sprintf("Film %d.en.srt", i)), "1\n00:00:01,000 --> 00:00:02,000\nHello\n")
		}
	}
	return server.New(server.Config{SubtitleApp: true, MediaDir: media, DataDir: t.TempDir(), CacheDir: t.TempDir()})
}

func TestSubtitleOverviewPrioritizesABoundedWantedPreview(t *testing.T) {
	t.Parallel()
	handler := subtitleOverviewServer(t, 7, false)
	overview := requestApp(t, handler, http.MethodGet, "/", "").Body.String()
	if strings.Count(overview, `class="subtitle-file"`) != 6 || !strings.Contains(overview, `data-wanted="7"`) || !strings.Contains(overview, `>View all wanted `) || strings.Contains(overview, `>Film 7</strong>`) {
		t.Fatalf("overview does not offer a bounded wanted preview: %s", overview)
	}
	ledger := strings.Index(overview, `class="subtitle-browser"`)
	readiness := strings.Index(overview, `id="subtitle-readiness-title"`)
	correction := strings.Index(overview, `id="subtitle-maintain-help"`)
	if ledger < 0 || readiness <= ledger || correction < 0 || correction >= ledger {
		t.Fatalf("wanted preview and blocking guidance do not precede operational details: ledger=%d readiness=%d correction=%d", ledger, readiness, correction)
	}
	if strings.Count(overview, `<summary>`) < 6 || !strings.Contains(overview, "Open subtitle editor") {
		t.Fatal("each file needs an accessible disclosure and editor link")
	}
}

func TestSubtitleOverviewPreviewKeepsOtherViewsAndAPIPagesAccessible(t *testing.T) {
	t.Parallel()
	handler := subtitleOverviewServer(t, 7, false)
	for _, path := range []string{"/?view=wanted", "/?view=library", "/?view=library&q=Film"} {
		body := requestApp(t, handler, http.MethodGet, path, "").Body.String()
		if strings.Count(body, `class="subtitle-file"`) != 7 || !strings.Contains(body, `>Film 7</strong>`) {
			t.Errorf("%s omitted matching files: %s", path, body)
		}
	}
	inventory := requestApp(t, handler, http.MethodGet, "/api/v1/subtitle-library?view=library", "")
	var data struct{ Items []struct{ ID string } }
	if err := json.Unmarshal(inventory.Body.Bytes(), &data); err != nil {
		t.Fatal(err)
	}
	if inventory.Code != http.StatusOK || len(data.Items) != 7 {
		t.Fatalf("API inventory was truncated: %d %s", inventory.Code, inventory.Body.String())
	}
}

func TestSubtitleOverviewShowsCoveredLibraryWithoutEmptyWarning(t *testing.T) {
	t.Parallel()
	handler := subtitleOverviewServer(t, 1, true)
	body := requestApp(t, handler, http.MethodGet, "/", "").Body.String()
	for _, text := range []string{"Your subtitles are ready.", "Your subtitles are up to date.", `value="100"`, `aria-valuetext="1 of 1 files ready"`} {
		if !strings.Contains(body, text) {
			t.Errorf("covered library lacks %q", text)
		}
	}
	if strings.Contains(body, "No video files found.") {
		t.Fatal("covered library was described as empty")
	}
}

func TestSubtitleOverviewOffersSetupForAnEmptyLibrary(t *testing.T) {
	t.Parallel()
	handler := subtitleOverviewServer(t, 0, false)
	body := requestApp(t, handler, http.MethodGet, "/", "").Body.String()
	if !strings.Contains(body, "Add your subtitle library") || !strings.Contains(body, `href="/settings#library">Add a media folder `) {
		t.Fatal("empty library lacks setup recovery")
	}
	if strings.Contains(body, "Your subtitles are ready.") || strings.Contains(body, "<meter") || strings.Contains(body, `action="/subtitles/manage/maintain"`) {
		t.Fatal("empty library claims coverage or offers a meaningless maintenance action")
	}
}
