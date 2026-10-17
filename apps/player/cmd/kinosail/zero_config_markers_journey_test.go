package main

import (
	"encoding/json"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"testing"
	"time"

	"github.com/MikeO7/kinosail/packages/commandtest"
	"github.com/MikeO7/kinosail/packages/servertest"
)

func TestZeroConfigurationServerAnalyzesAndAutoSkipsTheSyntheticSeason(t *testing.T) {
	if os.Getenv("KINOSAIL_MARKER_MEDIA_E2E") == "" || runtime.GOOS != "darwin" {
		t.Skip("set KINOSAIL_MARKER_MEDIA_E2E=1 on macOS to run the real-tool zero-configuration journey")
	}
	tools := map[string]string{}
	for _, name := range []string{"ffmpeg", "ffprobe", "fpcalc"} {
		path, err := exec.LookPath(name)
		if err != nil {
			t.Skipf("%s is unavailable", name)
		}
		tools[name] = path
	}
	season := os.Getenv("KINOSAIL_MARKER_MEDIA_DIR")
	if season == "" {
		season = t.TempDir()
	}
	if _, err := os.Stat(filepath.Join(season, "checksums.txt")); err != nil { //nolint:gosec // Operator-chosen fixture folder.
		script := filepath.Join("..", "..", "..", "..", "packages", "markers", "testdata", "synthetic-season.sh")
		if output, err := exec.CommandContext(t.Context(), "bash", script, season).CombinedOutput(); err != nil { //nolint:gosec // Checked-in fixture script.
			t.Fatalf("synthetic season: %v\n%s", err, output)
		}
	}
	media := t.TempDir()
	show := filepath.Join(media, "Shows", "Synthetic", "Season 01")
	if err := os.MkdirAll(show, 0o750); err != nil {
		t.Fatal(err)
	}
	for episode := 1; episode <= 3; episode++ {
		source := filepath.Join(season, "episode-"+strconv.Itoa(episode)+".mp4")
		data, err := os.ReadFile(source) //nolint:gosec // Rendered fixture inside the operator-chosen folder.
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(show, "Synthetic S01E0"+strconv.Itoa(episode)+".mp4"), data, 0o600); err != nil { //nolint:gosec // Episode copy inside the test media folder.
			t.Fatal(err)
		}
	}
	process := commandtest.StartServerProcess(t, "TestExitingServerHelperProcess", "KINOSAIL_MEDIA_DIR="+media, "KINOSAIL_CACHE_DIR="+t.TempDir(),
		"KINOSAIL_FFMPEG="+tools["ffmpeg"], "KINOSAIL_FFPROBE="+tools["ffprobe"], "KINOSAIL_FPCALC="+tools["fpcalc"])
	_, owner := servertest.SetupRouteAuthorization(t, process.Handler, setupToken)
	var settings struct {
		AutoSkip []string `json:"autoSkip"`
	}
	decodeRoute(t, servertest.RouteJSON(t, process.Handler, owner, http.MethodGet, "/api/v1/settings", nil), &settings)
	if !slices.Equal(settings.AutoSkip, []string{"intro", "recap", "commercial", "outro", "credits"}) {
		t.Fatalf("default auto skip = %#v", settings.AutoSkip)
	}
	var analysis struct {
		State string `json:"state"`
		Items int    `json:"items"`
		Error string `json:"error"`
	}
	started := time.Now()
	for deadline := time.Now().Add(4 * time.Minute); analysis.Items < 3 || analysis.State != "complete"; time.Sleep(time.Second) {
		if time.Now().After(deadline) {
			t.Fatalf("analysis did not finish on its own: %#v", analysis)
		}
		decodeRoute(t, servertest.RouteJSON(t, process.Handler, owner, http.MethodGet, "/api/v1/marker-analysis", nil), &analysis)
	}
	elapsed := time.Since(started)
	var library struct {
		Items []struct {
			ID      string `json:"id"`
			Episode int    `json:"episode"`
		} `json:"items"`
	}
	decodeRoute(t, servertest.RouteJSON(t, process.Handler, owner, http.MethodGet, "/api/v1/library", nil), &library)
	if len(library.Items) != 3 {
		t.Fatalf("library = %#v", library.Items)
	}
	results := map[string]any{}
	for _, item := range library.Items {
		var playback struct {
			AutoSkip []string `json:"autoSkip"`
			Markers  []struct {
				Type, Source string
				Start, End   float64
			} `json:"markers"`
		}
		decodeRoute(t, servertest.RouteJSON(t, process.Handler, owner, http.MethodGet, "/api/v1/items/"+item.ID+"/playback", nil), &playback)
		results[item.ID] = playback
		if !slices.Equal(playback.AutoSkip, []string{"intro", "outro", "credits"}) {
			t.Errorf("%s auto skip = %#v markers %#v", item.ID, playback.AutoSkip, playback.Markers)
		}
	}
	artifact, _ := json.MarshalIndent(map[string]any{"analysisWait": elapsed.String(), "analysis": analysis, "playback": results}, "", "  ")
	t.Logf("ZERO_CONFIGURATION_ARTIFACT %s", artifact)
}

func decodeRoute(t *testing.T, response interface{ Result() *http.Response }, target any) {
	t.Helper()
	result := response.Result()
	defer result.Body.Close()
	if result.StatusCode != http.StatusOK {
		t.Fatalf("route = %d", result.StatusCode)
	}
	if err := json.NewDecoder(result.Body).Decode(target); err != nil {
		t.Fatal(err)
	}
}
