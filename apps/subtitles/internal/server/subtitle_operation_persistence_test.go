package server_test

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/MikeO7/kinosail-subtitles/internal/server"
)

func TestSubtitlePreparedReceiptBecomesUnavailableAfterRealLifecycleRestart(t *testing.T) {
	config, target, calls, _ := subtitleOperationAudioConfig(t, false)
	firstLifecycle, stopFirst := context.WithCancel(t.Context())
	t.Cleanup(stopFirst)
	config.Lifecycle = firstLifecycle
	first := server.New(config)
	base := "/api/v1/subtitle-library/" + firstSubtitleInventoryID(t, first)
	prepared := prepareSubtitleOperation(t, first, base, "audio")
	stopFirst()
	config.Lifecycle = t.Context()
	restarted := server.New(config)
	assertSubtitleOperationUnavailable(t, restarted, base+"/audio", prepared.ID, `{"language":"en"}`)
	assertSubtitleOperationNoProcess(t, calls)
	assertSubtitleActionBytes(t, target, []byte(subtitleActionInitial))
	_ = subtitleActionHistory(t, restarted, nil, nil)
}

func TestSubtitleFailedActivationPersistenceNeverStartsWorkOrResurrectsAfterRestart(t *testing.T) {
	config, target, calls, _ := subtitleOperationAudioConfig(t, false)
	firstLifecycle, stopFirst := context.WithCancel(t.Context())
	t.Cleanup(stopFirst)
	config.Lifecycle = firstLifecycle
	first := server.New(config)
	base := "/api/v1/subtitle-library/" + firstSubtitleInventoryID(t, first)
	prepared := prepareSubtitleOperation(t, first, base, "audio")
	restoreRegistry := blockSubtitleOperationDurablePath(t, config.DataDir)
	activation := activateSubtitleOperationAfterStartup(t, first, base, prepared.ID, target, calls)
	if activation.Code != http.StatusServiceUnavailable {
		t.Fatalf("failed activation persistence = %d, want 503 before launch", activation.Code)
	}
	assertSubtitleOperationNoProcess(t, calls)
	assertSubtitleActionBytes(t, target, []byte(subtitleActionInitial))
	stopFirst()
	restoreRegistry()
	config.Lifecycle = t.Context()
	restarted := server.New(config)
	assertSubtitleOperationUnavailable(t, restarted, base+"/audio", prepared.ID, `{"language":"en"}`)
	assertSubtitleOperationNoProcess(t, calls)
	assertSubtitleActionBytes(t, target, []byte(subtitleActionInitial))
	_ = subtitleActionHistory(t, restarted, nil, nil)
}

func TestSubtitleCompletedRestoreOutcomeSurvivesRestartWithoutReplay(t *testing.T) {
	config, target, _, _ := subtitleOperationAudioConfig(t, false)
	firstLifecycle, stopFirst := context.WithCancel(t.Context())
	t.Cleanup(stopFirst)
	config.Lifecycle = firstLifecycle
	first := server.New(config)
	base := "/api/v1/subtitle-library/" + firstSubtitleInventoryID(t, first)
	subtitleOperationSetupSave(t, first, base)
	shifted, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	prepared := prepareSubtitleOperation(t, first, base, "restore")
	assertSubtitleOperationAccepted(t, activateSubtitleOperation(t, first, base+"/restore", `{"language":"en"}`, []string{prepared.ID}), prepared.ID)
	_ = waitSubtitleOperation(t, first, prepared.ID)
	stopFirst()
	config.Lifecycle = t.Context()
	restarted := server.New(config)
	completed := readSubtitleOperation(t, restarted, prepared.ID)
	if completed.State != "completed" || completed.Status != http.StatusNoContent || completed.Outcome != "success" {
		t.Fatalf("completed outcome after restart = %+v", completed)
	}
	assertSubtitleOperationAccepted(t, activateSubtitleOperation(t, restarted, base+"/restore", `{"language":"en"}`, []string{prepared.ID}), prepared.ID)
	assertSubtitleActionBytes(t, target, []byte(subtitleActionInitial))
	assertSubtitleActionBytes(t, target+".kinosail.bak", shifted)
	_ = subtitleActionHistory(t, restarted, []string{"restored", "updated"}, []string{"restore", "manual"})
}

func assertSubtitleOperationUnavailable(t *testing.T, handler http.Handler, path, id, body string) {
	t.Helper()
	status := requestApp(t, handler, http.MethodGet, "/api/v1/subtitle-operations/"+id, "")
	if status.Code != http.StatusNotFound {
		t.Fatalf("prior prepared receipt status = %d, want unavailable", status.Code)
	}
	activation := activateSubtitleOperation(t, handler, path, body, []string{id})
	if activation.Code != http.StatusNotFound {
		t.Fatalf("prior prepared receipt activation = %d, want rejection before work", activation.Code)
	}
}

func assertSubtitleOperationNoProcess(t *testing.T, calls string) {
	t.Helper()
	if _, err := os.Stat(calls); !os.IsNotExist(err) {
		t.Fatalf("rejected activation started a fictional analysis process: %v", err)
	}
}

// This is a file-system fault at the durable boundary, not a replacement of
// the registry. The previous bytes and blocking directory both remain intact.
func blockSubtitleOperationDurablePath(t *testing.T, data string) func() {
	t.Helper()
	path := filepath.Join(data, "subtitle_operations.json")
	snapshot := path + ".before-fault"
	if err := os.Rename(path, snapshot); err != nil {
		t.Fatalf("durable-boundary fixture prerequisite: %v", err)
	}
	if err := os.Mkdir(path, 0o700); err != nil {
		t.Fatal(err)
	}
	return func() {
		if err := os.Rename(path, path+".blocking-directory"); err != nil {
			t.Fatal(err)
		}
		if err := os.Rename(snapshot, path); err != nil {
			t.Fatal(err)
		}
	}
}
