package server

import (
	"context"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/MikeO7/kinosail-subtitles/internal/configuration"
)

func TestLocalSubtitleDraftStopsWhenServerStops(t *testing.T) {
	t.Parallel()
	facts := `{"format":{"duration":"10"},"streams":[{"index":2,"codec_type":"subtitle","codec_name":"hdmv_pgs_subtitle","tags":{"language":"eng"}}]}`
	manager, item := subtitleFactsFixture(t, "#!/bin/sh\nprintf '%s' '"+facts+"'\n")
	manager.index.SetRoots([]libraryRoot{{Path: filepath.Dir(item.Path)}})
	manager.probe.ffmpeg = subtitleProcessFixture(t, "exec sleep 60")
	tool := subtitleProcessFixture(t, "exit 1")
	config, err := configuration.Load(t.TempDir(), "", func(key string) (string, bool) {
		if key == "KINOSAIL_TESSERACT" {
			return tool, true
		}
		return "", false
	})
	if err != nil {
		t.Fatal(err)
	}
	manager.settings.config = config
	lifecycle, stop := context.WithCancel(t.Context())
	defer stop()
	manager.operations = newSubtitleOperations(Config{Lifecycle: lifecycle, DataDir: t.TempDir()})
	request := httptest.NewRequestWithContext(withViewer(t.Context(), viewerProfile{ID: "owner", Owner: true}), "POST", "/", nil)
	draft, status, err := manager.localSubtitleDraft(request, item.ID, subtitleDraftInput{Language: "en", Method: "ocr", Action: "start"})
	if err != nil || status != 202 || draft.State != "running" {
		t.Fatalf("start=%#v %d %v", draft, status, err)
	}
	stop()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		manager.drafts.Lock()
		state := manager.drafts.current.State
		manager.drafts.Unlock()
		if state == "stopped" {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("local draft kept running after the server stopped")
}
