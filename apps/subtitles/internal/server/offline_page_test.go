package server_test

import (
	"net/http"
	"regexp"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail-subtitles/internal/server"
)

func TestOfflinePageLoadsOnlyServedAssetsAndOffersRetry(t *testing.T) {
	t.Parallel()
	handler := server.New(server.Config{SubtitleApp: true, MediaDir: t.TempDir(), DataDir: t.TempDir(), CacheDir: t.TempDir()})
	page := requestApp(t, handler, http.MethodGet, "/offline", "")
	body := page.Body.String()
	if page.Code != http.StatusOK {
		t.Fatalf("offline page = %d %q", page.Code, body)
	}
	if !strings.Contains(body, `href="/">Try again</a>`) {
		t.Errorf("offline page has no retry link: %q", body)
	}
	references := regexp.MustCompile(`(?:src|href)="(/static/[^"]+)"`).FindAllStringSubmatch(body, -1)
	if len(references) == 0 {
		t.Fatalf("offline page has no static assets: %q", body)
	}
	for _, reference := range references {
		if asset := requestApp(t, handler, http.MethodGet, reference[1], ""); asset.Code != http.StatusOK {
			t.Errorf("offline asset %s = %d", reference[1], asset.Code)
		}
	}
	if downloads := requestApp(t, handler, http.MethodGet, "/downloads", ""); downloads.Code != http.StatusNotFound {
		t.Fatalf("Subtitles downloads route = %d", downloads.Code)
	}
	for _, unavailable := range []string{"Offline downloads", "Checking this device", "data-offline-library"} {
		if strings.Contains(body, unavailable) {
			t.Errorf("offline page waits for device downloads Subtitles does not provide: %q", unavailable)
		}
	}
}
