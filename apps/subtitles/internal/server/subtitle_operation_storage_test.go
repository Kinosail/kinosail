package server_test

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail-subtitles/internal/server"
)

func TestSubtitleCorruptOperationRegistryFailsClosedWithoutChangingLegacyOwnerWorkflow(t *testing.T) {
	cases := []struct{ name, data string }{
		{"malformed", `{`},
		{"unsupported version", `{"version":2,"receipts":[]}`},
		{"duplicate version", `{"version":1,"version":1,"receipts":[]}`},
		{"unknown field", `{"version":1,"receipts":[],"allowUnknown":true}`},
		{"oversized", strings.Repeat(" ", 96*1024+1)},
		{"invalid receipt identity", `{"version":1,"receipts":[{"id":"bad"}]}`},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			config, target, calls, _ := subtitleOperationAudioConfig(t, false)
			path := filepath.Join(config.DataDir, "subtitle_operations.json")
			writeTestFile(t, path, test.data)
			handler := server.New(config)
			base := "/api/v1/subtitle-library/" + firstSubtitleInventoryID(t, handler)
			item := strings.TrimPrefix(base, "/api/v1/subtitle-library/")
			prepared := requestJSON(t, handler, http.MethodPost, "/api/v1/subtitle-operations", `{"action":"audio","item":"`+item+`"}`)
			if prepared.Code != http.StatusServiceUnavailable {
				t.Fatalf("untrusted durable registry preparation = %d, want safe503", prepared.Code)
			}
			assertSubtitleOperationNoProcess(t, calls)
			assertSubtitleActionBytes(t, target, []byte(subtitleActionInitial))
			assertSubtitleActionBytes(t, path, []byte(test.data))
			_ = subtitleActionHistory(t, handler, nil, nil)
			// A failed receipt service cannot remove existing Owner capabilities.
			subtitleOperationSetupSave(t, handler, base)
			assertSubtitleActionBytes(t, target+".kinosail.bak", []byte(subtitleActionInitial))
			_ = subtitleActionHistory(t, handler, []string{"updated"}, []string{"manual"})
		})
	}
}

func TestSubtitleOperationPreparationPersistenceFailureStartsNoWork(t *testing.T) {
	config, target, calls, _ := subtitleOperationAudioConfig(t, false)
	path := filepath.Join(config.DataDir, "subtitle_operations.json")
	if err := os.Mkdir(path, 0o700); err != nil {
		t.Fatal(err)
	}
	handler := server.New(config)
	base := "/api/v1/subtitle-library/" + firstSubtitleInventoryID(t, handler)
	item := strings.TrimPrefix(base, "/api/v1/subtitle-library/")
	prepared := requestJSON(t, handler, http.MethodPost, "/api/v1/subtitle-operations", `{"action":"audio","item":"`+item+`"}`)
	if prepared.Code != http.StatusServiceUnavailable {
		t.Fatalf("unavailable preparation persistence = %d, want503 before any worker", prepared.Code)
	}
	assertSubtitleOperationNoProcess(t, calls)
	assertSubtitleActionBytes(t, target, []byte(subtitleActionInitial))
	_ = subtitleActionHistory(t, handler, nil, nil)
}
