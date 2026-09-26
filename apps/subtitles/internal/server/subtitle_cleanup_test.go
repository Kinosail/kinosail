package server

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail/packages/library"
)

func TestSubtitleCleanupPreviewsAndRemovesOnlySelectedSidecars(t *testing.T) {
	dir := t.TempDir()
	media := filepath.Join(dir, "Film.mkv")
	if err := os.WriteFile(media, []byte("video"), 0o600); err != nil {
		t.Fatal(err)
	}
	files := []string{"Film.en.srt", "Film.en.forced.srt", "Film.forced.en.vtt", "Film.es.srt", "Film.es.forced.srt", "Film.fr.vtt", "Film.srt", "Film.commentary.srt"}
	item := library.Item{ID: "0123456789abcdef", Kind: "video", Path: media}
	for _, name := range files {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(name), 0o600); err != nil {
			t.Fatal(err)
		}
		item.Subtitles = append(item.Subtitles, path)
	}
	index := sidecarTestIndex(item)
	settings := &settingsStore{file: "settings.json", value: installationSettings{SubtitleLanguage: "en", SubtitleLanguages: []string{"en", "es"}}, persist: func(string, any) error { return nil }}
	plan, err := planSubtitleCleanup(index, "en", "keep")
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Files) != 3 || plan.Skipped != 2 {
		t.Fatalf("preview: files=%v skipped=%d", plan.Files, plan.Skipped)
	}
	if _, err := applySubtitleCleanup(index, settings, "en", "keep", strings.Repeat("0", 64)); err == nil {
		t.Fatal("stale preview accepted")
	}
	if !slices.Equal(settings.subtitleLanguages(), []string{"en", "es"}) {
		t.Fatal("stale preview changed preferences")
	}
	for _, name := range files {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Fatalf("stale preview removed %s: %v", name, err)
		}
	}
	removed, err := applySubtitleCleanup(index, settings, "en", "keep", plan.Digest)
	if err != nil || removed != 3 {
		t.Fatalf("apply: removed=%d err=%v", removed, err)
	}
	if !slices.Equal(settings.subtitleLanguages(), []string{"en"}) {
		t.Fatalf("preferences after cleanup: %q", settings.subtitleLanguages())
	}
	for _, name := range files {
		_, err := os.Stat(filepath.Join(dir, name))
		wantRemoved := strings.Contains(name, ".es.") || strings.Contains(name, ".fr.")
		if (err != nil) != wantRemoved {
			t.Errorf("%s: err=%v wantRemoved=%v", name, err, wantRemoved)
		}
	}
	plan, err = planSubtitleCleanup(index, "en", "delete")
	if err != nil || len(plan.Files) != 2 {
		t.Fatalf("forced preview: %v %v", plan, err)
	}
	removed, err = applySubtitleCleanup(index, settings, "en", "delete", plan.Digest)
	if err != nil || removed != 2 {
		t.Fatalf("forced apply: %d %v", removed, err)
	}
	if _, err := os.Stat(filepath.Join(dir, "Film.en.srt")); err != nil {
		t.Fatal("dialogue subtitle removed")
	}
}

func TestSubtitleCleanupRejectsInvalidPolicyAndChangedFiles(t *testing.T) {
	dir := t.TempDir()
	media, sidecar := filepath.Join(dir, "Film.mkv"), filepath.Join(dir, "Film.es.srt")
	if err := os.WriteFile(media, []byte("video"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(sidecar, []byte("hola"), 0o600); err != nil {
		t.Fatal(err)
	}
	index := sidecarTestIndex(library.Item{Kind: "video", Path: media, Subtitles: []string{sidecar}})
	settings := &settingsStore{file: "settings.json", value: installationSettings{SubtitleLanguage: "en"}, persist: func(string, any) error { return nil }}
	for _, input := range [][2]string{{"", "keep"}, {"invalid", "keep"}, {"en", ""}, {"en", "other"}, {strings.Repeat("x", 100), "keep"}} {
		if _, err := planSubtitleCleanup(index, input[0], input[1]); err == nil {
			t.Errorf("accepted %q", input)
		}
	}
	plan, err := planSubtitleCleanup(index, "en", "keep")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(sidecar, []byte("changed after preview"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := applySubtitleCleanup(index, settings, "en", "keep", plan.Digest); err == nil {
		t.Fatal("changed file removed")
	}
	if _, err := os.Stat(sidecar); err != nil {
		t.Fatal("changed file removed")
	}
}

func TestSubtitleCleanupLeavesSymlinksAndPathsOutsideTheVideoDirectory(t *testing.T) {
	dir := t.TempDir()
	media := filepath.Join(dir, "Film.mkv")
	outside := filepath.Join(t.TempDir(), "outside.srt")
	for _, path := range []string{media, outside} {
		if err := os.WriteFile(path, []byte("fixture"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	linked := filepath.Join(dir, "Film.es.srt")
	if err := os.Symlink(outside, linked); err != nil {
		t.Fatal(err)
	}
	index := sidecarTestIndex(library.Item{Kind: "video", Path: media, Subtitles: []string{linked, outside}})
	settings := &settingsStore{file: "settings.json", value: installationSettings{SubtitleLanguage: "en"}, persist: func(string, any) error { return nil }}
	plan, err := planSubtitleCleanup(index, "en", "keep")
	if err != nil || len(plan.Files) != 0 || plan.Skipped != 2 {
		t.Fatalf("plan=%+v err=%v", plan, err)
	}
	removed, err := applySubtitleCleanup(index, settings, "en", "keep", plan.Digest)
	if err != nil || removed != 0 {
		t.Fatalf("removed=%d err=%v", removed, err)
	}
	if _, err := os.Stat(outside); err != nil {
		t.Fatalf("outside file changed: %v", err)
	}
}

func TestSubtitleCleanupWebPreviewAndDelete(t *testing.T) {
	dir := t.TempDir()
	media, sidecar := filepath.Join(dir, "Film.mkv"), filepath.Join(dir, "Film.es.srt")
	for _, path := range []string{media, sidecar} {
		if err := os.WriteFile(path, []byte("fixture"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	index := sidecarTestIndex(library.Item{Kind: "video", Path: media, Subtitles: []string{sidecar}})
	settings := &settingsStore{file: "settings.json", value: installationSettings{SubtitleLanguage: "en"}, persist: func(string, any) error { return nil }}
	preview := previewSubtitleCleanup(index, settings)
	for _, query := range []string{"", "?language=invalid&forced=keep", "?language=en&forced=keep&extra=1", "?language=en&language=fr&forced=keep", "?language=en&forced=other", "?language=en&forced=keep&bad=%ZZ"} {
		response := httptest.NewRecorder()
		preview(response, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/settings/subtitles/cleanup"+query, nil))
		if response.Code != http.StatusBadRequest {
			t.Errorf("%q: %d", query, response.Code)
		}
	}
	response := httptest.NewRecorder()
	preview(response, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/settings/subtitles/cleanup?language=en&forced=keep", nil))
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "1 subtitle file to delete") {
		t.Fatalf("preview: %d %s", response.Code, response.Body.String())
	}
	match := regexp.MustCompile(`name="digest" value="([0-9a-f]{64})"`).FindStringSubmatch(response.Body.String())
	if len(match) != 2 {
		t.Fatal("preview digest missing")
	}
	apply := deleteSubtitleCleanup(index, settings)
	for _, body := range []string{"language=en&forced=keep", "language=en&forced=keep&digest=" + match[1] + "&extra=1", "language=en&forced=delete&digest=" + match[1]} {
		request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/settings/subtitles/cleanup", strings.NewReader(body))
		request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		result := httptest.NewRecorder()
		apply(result, request)
		if result.Code == http.StatusOK {
			t.Errorf("accepted %q", body)
		}
	}
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/settings/subtitles/cleanup", strings.NewReader("language=en&forced=keep&digest="+match[1]+"&_csrf=one&_csrf=two"))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	duplicate := httptest.NewRecorder()
	apply(duplicate, request)
	if duplicate.Code != http.StatusBadRequest {
		t.Fatalf("duplicate CSRF field = %d", duplicate.Code)
	}
	if _, err := os.Stat(sidecar); err != nil {
		t.Fatal("invalid request deleted subtitle")
	}
	request = httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/settings/subtitles/cleanup", strings.NewReader("language=en&forced=keep&digest="+match[1]+"&_csrf=fixture"))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	result := httptest.NewRecorder()
	apply(result, request)
	if result.Code != http.StatusOK || !strings.Contains(result.Body.String(), "Deleted 1 subtitle file") {
		t.Fatalf("apply: %d %s", result.Code, result.Body.String())
	}
	if _, err := os.Stat(sidecar); !os.IsNotExist(err) {
		t.Fatalf("subtitle remains: %v", err)
	}
}
