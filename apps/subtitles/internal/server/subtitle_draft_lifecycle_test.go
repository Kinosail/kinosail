package server

import (
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail-subtitles/internal/configuration"
)

func TestLocalSubtitleDraftRejectsUnavailableDependenciesWithoutStarting(t *testing.T) {
	t.Parallel()
	for _, scenario := range []string{"closed", "running", "no probe", "no ffmpeg", "duration", "track", "tool", "ffmpeg"} {
		t.Run(scenario, func(t *testing.T) {
			assertDraftDependencyRejected(t, scenario)
		})
	}
}

func TestLocalSubtitleDraftCancellationAndMissingItem(t *testing.T) {
	t.Parallel()
	manager, item := subtitleFactsFixture(t, "#!/bin/sh\nexit 1\n")
	manager.index.SetRoots([]libraryRoot{{Path: filepath.Dir(item.Path)}})
	request := httptest.NewRequestWithContext(withViewer(t.Context(), viewerProfile{ID: "owner", Owner: true}), "POST", "/", nil)
	input := subtitleDraftInput{Language: "en", Method: "ocr", Action: "start"}
	if _, status, err := manager.localSubtitleDraft(request, "ffffffffffffffff", input); status != 404 || err == nil {
		t.Fatalf("missing=%d %v", status, err)
	}
	canceled := false
	manager.drafts.current = subtitleDraft{ID: strings.Repeat("a", 64), Item: item.ID, Language: "en", Method: "ocr", cancel: func() { canceled = true }}
	input.Action = "cancel"
	input.DraftID = manager.drafts.current.ID
	draft, status, err := manager.localSubtitleDraft(request, item.ID, input)
	if err != nil || status != 200 || !canceled || draft.State != "canceling" {
		t.Fatalf("cancel=%#v %d %v", draft, status, err)
	}
}

func assertDraftDependencyRejected(t *testing.T, scenario string) {
	t.Helper()

	facts := `{"format":{"duration":"10"},"streams":[{"index":2,"codec_type":"subtitle","codec_name":"hdmv_pgs_subtitle","tags":{"language":"eng"}}]}`
	if scenario == "duration" {
		facts = `{"format":{"duration":"0"}}`
	}
	if scenario == "track" {
		facts = `{"format":{"duration":"10"}}`
	}
	manager, item := subtitleFactsFixture(t, "#!/bin/sh\nprintf '%s' '"+facts+"'\n")
	manager.index.SetRoots([]libraryRoot{{Path: filepath.Dir(item.Path)}})
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
	want := configureUnavailableDraft(manager, scenario)
	request := httptest.NewRequestWithContext(withViewer(t.Context(), viewerProfile{ID: "owner", Owner: true}), "POST", "/", nil)
	_, status, err := manager.localSubtitleDraft(request, item.ID, subtitleDraftInput{Language: "en", Method: "ocr", Action: "start"})
	if status != want || err == nil || manager.drafts.current.ID != "" {
		t.Fatalf("draft=%#v status=%d err=%v", manager.drafts.current, status, err)
	}
}

func configureUnavailableDraft(manager *subtitleManager, scenario string) int {
	want := 409
	switch scenario {
	case "closed":
		manager.drafts.closed = true
		want = 503
	case "running":
		manager.drafts.current.cancel = func() {}
	case "no probe":
		manager.probe = nil
	case "no ffmpeg":
		manager.probe.ffmpeg = ""
	case "tool":
		manager.settings.config = configuration.Snapshot{}
	}
	return want
}
