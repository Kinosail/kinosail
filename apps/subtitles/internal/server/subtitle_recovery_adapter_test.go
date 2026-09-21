package server_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail-subtitles/internal/server"
)

func TestSubtitleRecoveryWorksThroughWebAndVersionedAPI(t *testing.T) { //nolint:cyclop // Web policy and API restore use the same exact-byte recovery operation.
	t.Parallel()
	media := t.TempDir()
	video := filepath.Join(media, "Arrival.mp4")
	target := filepath.Join(media, "Arrival.en.srt")
	backup := target + ".kinosail.bak"
	current := "1\n00:00:01,000 --> 00:00:02,000\nCurrent\n"
	previous := "1\n00:00:01,000 --> 00:00:02,000\nPrevious\n"
	writeTestFile(t, video, "video")
	writeTestFile(t, target, current)
	writeTestFile(t, backup, previous)
	handler := server.New(server.Config{SubtitleApp: true, MediaDir: media, DataDir: t.TempDir(), CacheDir: t.TempDir()})
	var inventory struct {
		Items []struct {
			ID     string `json:"id"`
			Frozen bool   `json:"frozen"`
		} `json:"items"`
	}
	response := requestApp(t, handler, http.MethodGet, "/api/v1/subtitle-library?view=library", "")
	if json.Unmarshal(response.Body.Bytes(), &inventory) != nil || len(inventory.Items) != 1 {
		t.Fatalf("inventory = %d %q", response.Code, response.Body.String())
	}
	id := inventory.Items[0].ID
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/subtitles/manage/"+id+"/replacement", strings.NewReader("replaceable=false"))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	web := httptest.NewRecorder()
	handler.ServeHTTP(web, request)
	if web.Code != http.StatusSeeOther {
		t.Fatalf("web replacement = %d %q", web.Code, web.Body.String())
	}
	response = requestApp(t, handler, http.MethodGet, "/api/v1/subtitle-library?view=library", "")
	if json.Unmarshal(response.Body.Bytes(), &inventory) != nil || !inventory.Items[0].Frozen {
		t.Fatalf("frozen inventory = %d %q", response.Code, response.Body.String())
	}
	replaceable := requestJSON(t, handler, http.MethodPost, "/api/v1/subtitle-library/"+id+"/replacement", `{"replaceable":true}`)
	if replaceable.Code != http.StatusNoContent {
		t.Fatalf("API replacement = %d %q", replaceable.Code, replaceable.Body.String())
	}
	restored := requestJSON(t, handler, http.MethodPost, "/api/v1/subtitle-library/"+id+"/restore", `{}`)
	restoredData, restoredErr := os.ReadFile(target)
	backupData, backupErr := os.ReadFile(backup)
	if restored.Code != http.StatusNoContent || restoredErr != nil || backupErr != nil || string(restoredData) != previous || string(backupData) != current {
		t.Fatalf("restore = %d %q, target = %q %v, backup = %q %v", restored.Code, restored.Body.String(), restoredData, restoredErr, backupData, backupErr)
	}
	library := requestApp(t, handler, http.MethodGet, "/?view=library", "")
	if !strings.Contains(library.Body.String(), "Restore previous") || !strings.Contains(library.Body.String(), "Allow automatic upgrades") || !strings.Contains(library.Body.String(), "Restored previous subtitle") {
		t.Fatalf("recovery history = %d %q", library.Code, library.Body.String())
	}
	webRestore := requestApp(t, handler, http.MethodPost, "/subtitles/manage/"+id+"/restore", "")
	if webRestore.Code != http.StatusSeeOther {
		t.Fatalf("web restore = %d %q", webRestore.Code, webRestore.Body.String())
	}
}

func TestSubtitleReadinessAndProviderHealthAreAvailableThroughTheAPI(t *testing.T) { //nolint:cyclop // One adapter test verifies every readiness and provider-health field.
	t.Parallel()
	media := t.TempDir()
	writeTestFile(t, filepath.Join(media, "Arrival.mp4"), "video")
	handler := server.New(server.Config{SubtitleApp: true, MediaDir: media, DataDir: t.TempDir(), CacheDir: t.TempDir()})
	var status struct {
		Readiness struct {
			State      string `json:"state"`
			Correction string `json:"correction"`
			Checks     []struct {
				Name     string `json:"name"`
				Blocking bool   `json:"blocking"`
			} `json:"checks"`
		} `json:"readiness"`
		Providers []struct {
			Name, State string
		} `json:"providers"`
	}
	response := requestApp(t, handler, http.MethodGet, "/api/v1/subtitle-library", "")
	if json.Unmarshal(response.Body.Bytes(), &status) != nil || status.Readiness.State != "Needs one correction" || len(status.Readiness.Checks) != 7 || status.Readiness.Checks[6].Name != "Last successful subtitle write" || status.Readiness.Checks[6].Blocking || len(status.Providers) != 3 {
		t.Fatalf("readiness = %d %q", response.Code, response.Body.String())
	}
	tested := requestJSON(t, handler, http.MethodPost, "/api/v1/subtitle-providers/test", `{}`)
	if tested.Code != http.StatusOK || !strings.Contains(tested.Body.String(), `"attempted":0`) || !strings.Contains(tested.Body.String(), `"connected":0`) {
		t.Fatalf("provider test = %d %q", tested.Code, tested.Body.String())
	}
	webTest := requestApp(t, handler, http.MethodPost, "/subtitles/providers/test", "")
	if webTest.Code != http.StatusSeeOther {
		t.Fatalf("web provider test = %d %q", webTest.Code, webTest.Body.String())
	}
}
