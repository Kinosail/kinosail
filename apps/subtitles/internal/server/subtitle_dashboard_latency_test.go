package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/MikeO7/kinosail/packages/library"
)

func subtitleFactsFixture(t *testing.T, script string) (*subtitleManager, library.Item) {
	t.Helper()
	root := t.TempDir()
	item := library.Item{ID: "0123456789abcdef", Title: "Film", Kind: "video", Path: filepath.Join(root, "Film.mkv")}
	if err := os.WriteFile(item.Path, []byte("video"), 0o600); err != nil {
		t.Fatal(err)
	}
	item = scannedSubtitleFactsItem(t, item)
	executable := filepath.Join(root, "ffprobe")
	writeSubtitleFactsExecutable(t, executable, script)
	index := memoryLibraryIndex([]library.Item{item}, true)
	settings := newSettingsStore(root, t.TempDir(), "", nil)
	probe := newMediaProbe(executable)
	probe.cacheDir, probe.ffmpeg = t.TempDir(), "unused"
	provider := newSubtitleProvider(SubtitleConfig{}, probe.cacheDir, t.TempDir(), index, settings, "")
	return newSubtitleManager(index, settings, provider, probe), item
}

func subtitleFactsAPI(t *testing.T, manager *subtitleManager) subtitleDashboardData {
	t.Helper()
	response := httptest.NewRecorder()
	manager.statusAPI(response, ownerRequest("/api/v1/subtitle-library?view=library"))
	var data subtitleDashboardData
	if response.Code != http.StatusOK || json.Unmarshal(response.Body.Bytes(), &data) != nil {
		t.Fatalf("inventory = %d %q", response.Code, response.Body.String())
	}
	if data.Total != data.Ready+data.Wanted+data.Pending+data.Unavailable {
		t.Fatalf("coverage counts do not partition the library: %#v", data)
	}
	return data
}

func TestSubtitleDashboardDoesNotWaitForBackgroundTrackChecks(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	started, release := filepath.Join(root, "started"), filepath.Join(root, "release")
	script := "#!/bin/sh\ntouch '" + started + "'\nwhile [ ! -f '" + release + "' ]; do sleep 0.01; done\nprintf '%s' '{\"streams\":[{\"index\":2,\"codec_type\":\"subtitle\",\"codec_name\":\"subrip\",\"tags\":{\"language\":\"eng\"}}]}'\n"
	manager, _ := subtitleFactsFixture(t, script)
	if data := subtitleFactsAPI(t, manager); data.Pending != 1 || data.Wanted != 0 || !data.Items[0].Pending {
		t.Fatalf("cold cache mislabeled as missing: %#v", data)
	}
	if _, err := os.Stat(started); !os.IsNotExist(err) {
		t.Fatalf("HTTP read launched a probe: %v", err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan struct{})
	go func() { defer close(done); manager.refreshFacts(ctx) }()
	t.Cleanup(func() { cancel(); <-done })
	deadline := time.Now().Add(3 * time.Second)
	for {
		if _, err := os.Stat(started); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("background probe did not start")
		}
		time.Sleep(10 * time.Millisecond)
	}
	responded := make(chan *httptest.ResponseRecorder, 1)
	go func() {
		response := httptest.NewRecorder()
		manager.dashboard(response, ownerRequest("/"))
		responded <- response
	}()
	select {
	case response := <-responded:
		if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `data-subtitle-pending="1"`) || strings.Contains(response.Body.String(), "Your subtitles are ready") {
			t.Fatalf("pending HTML = %d %q", response.Code, response.Body.String())
		}
	case <-time.After(time.Second):
		t.Fatal("dashboard waited for an in-flight probe")
	}
	if err := os.WriteFile(release, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("track check did not finish")
	}
	data := subtitleFactsAPI(t, manager)
	if data.Pending != 0 || data.Ready != 1 || !strings.Contains(data.Items[0].Tracks, "Embedded en") || !data.Items[0].NeedsSidecar {
		t.Fatalf("completed inventory = %#v", data)
	}
}

func TestSubtitleDashboardTracksFailuresAndRecoversAfterSourceChange(t *testing.T) {
	t.Parallel()
	manager, item := subtitleFactsFixture(t, "#!/bin/sh\nexit 1\n")
	manager.refreshFacts(t.Context())
	data := subtitleFactsAPI(t, manager)
	if data.Pending != 0 || data.Unavailable != 1 || data.Wanted != 0 || !data.Items[0].Unavailable {
		t.Fatalf("failed probe was treated as known coverage: %#v", data)
	}
	if err := os.WriteFile(item.Path, []byte("new media version"), 0o600); err != nil {
		t.Fatal(err)
	}
	item = scannedSubtitleFactsItem(t, item)
	manager.index = memoryLibraryIndex([]library.Item{item}, true)
	if data = subtitleFactsAPI(t, manager); data.Pending != 1 || data.Unavailable != 0 {
		t.Fatalf("old failure applied to replacement: %#v", data)
	}
	writeSubtitleFactsExecutable(t, manager.probe.executable, "#!/bin/sh\nprintf '%s' '{\"format\":{\"format_name\":\"mkv\"}}'\n")
	manager.refreshFacts(t.Context())
	if data = subtitleFactsAPI(t, manager); data.Pending != 0 || data.Unavailable != 0 || data.Wanted != 1 {
		t.Fatalf("retry did not recover: %#v", data)
	}
}

func scannedSubtitleFactsItem(t *testing.T, item library.Item) library.Item {
	t.Helper()
	info, err := os.Stat(item.Path)
	if err != nil {
		t.Fatal(err)
	}
	item.Size, item.Added = info.Size(), info.ModTime()
	return item
}

func TestSubtitleDashboardUsesLastScanWithoutRevalidatingMedia(t *testing.T) {
	t.Parallel()
	manager, item := subtitleFactsFixture(t, "#!/bin/sh\nprintf '%s' '{\"streams\":[{\"index\":2,\"codec_type\":\"subtitle\",\"codec_name\":\"subrip\",\"tags\":{\"language\":\"eng\"}}]}'\n")
	manager.refreshFacts(t.Context())
	if err := os.Remove(item.Path); err != nil {
		t.Fatal(err)
	}
	if data := subtitleFactsAPI(t, manager); data.Ready != 1 || data.Pending != 0 {
		t.Fatalf("dashboard revalidated media instead of using the scan: %#v", data)
	}
	manager.index = memoryLibraryIndex(nil, true)
	if data := subtitleFactsAPI(t, manager); data.Total != 0 || len(data.Items) != 0 {
		t.Fatalf("removed scan item remained visible: %#v", data)
	}
}

func TestSubtitleDashboardInvalidQueryDoesNotStartTrackChecks(t *testing.T) {
	t.Parallel()
	calls := filepath.Join(t.TempDir(), "calls")
	manager, _ := subtitleFactsFixture(t, "#!/bin/sh\ntouch '"+calls+"'\n")
	for _, path := range []string{"/?unexpected=1", "/?view=unknown", "/?view=library&view=wanted", "/?q=" + strings.Repeat("a", 129)} {
		for _, handler := range []http.HandlerFunc{manager.dashboard, manager.statusAPI} {
			response := httptest.NewRecorder()
			handler(response, ownerRequest(path))
			if response.Code != http.StatusBadRequest {
				t.Errorf("%s = %d", path, response.Code)
			}
		}
	}
	if _, err := os.Stat(calls); !os.IsNotExist(err) {
		t.Fatalf("invalid query started a track check: %v", err)
	}
}

func writeSubtitleFactsExecutable(t *testing.T, path, script string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
}
