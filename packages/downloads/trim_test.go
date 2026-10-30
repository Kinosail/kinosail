package downloads

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const trimSize = 400

func trimJob(t *testing.T, root, id, state string, created time.Time) Job {
	t.Helper()
	content := []byte(strings.Repeat("v", trimSize))
	path := filepath.Join(root, id+".mp4")
	for name, data := range map[string][]byte{path: content, path + ".manifest": []byte("{}"), filepath.Join(root, id+".json"): []byte("{}")} {
		if err := os.WriteFile(name, data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Chtimes(path, created, created); err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(content)
	return Job{ID: id, Profile: "viewer", State: state, File: path, Size: trimSize, SHA256: hex.EncodeToString(digest[:]), ReadyOffline: state == "ready", Created: created}
}

func gone(paths ...string) bool {
	for _, path := range paths {
		if _, err := os.Lstat(path); err == nil {
			return false
		}
	}
	return true
}

func TestTrimRemovesLeastRecentlyUsedReadyDownloadsUntilUnderLimit(t *testing.T) {
	t.Parallel()
	root, now := t.TempDir(), time.Now()
	oldest := trimJob(t, root, "aaaaaaaaaaaaaaaa", "ready", now.Add(-30*24*time.Hour))
	older := trimJob(t, root, "bbbbbbbbbbbbbbbb", "ready", now.Add(-10*24*time.Hour))
	recent := trimJob(t, root, "cccccccccccccccc", "ready", now.Add(-time.Hour))
	var published []string
	manager := &Manager{
		root: root, jobs: map[string]Job{oldest.ID: oldest, older.ID: older, recent.ID: recent},
		attempts: map[string]*attempt{}, restoring: map[string]Job{}, manifestWork: map[string]*manifestFlight{},
		publish: func(job Job) { published = append(published, job.ID) },
	}

	result, err := manager.Trim(900, now)
	if err != nil {
		t.Fatal(err)
	}
	if !gone(oldest.File, oldest.File+".manifest", filepath.Join(root, oldest.ID+".json")) || gone(older.File) || gone(recent.File) {
		t.Fatalf("oldest gone=%t older gone=%t recent gone=%t", gone(oldest.File), gone(older.File), gone(recent.File))
	}
	if _, found := manager.Get("viewer", oldest.ID); found || len(published) != 1 || published[0] != oldest.ID {
		t.Fatalf("job still listed=%t published=%v", found, published)
	}
	if result.Removed != 1 || result.FreedBytes != 400 || result.Bytes != 800 {
		t.Fatalf("result = %#v", result)
	}
}

func TestTrimKeepsPreparingAndRecentlyDeliveredDownloads(t *testing.T) {
	t.Parallel()
	root, now := t.TempDir(), time.Now()
	preparing := trimJob(t, root, "aaaaaaaaaaaaaaaa", "preparing", now.Add(-30*24*time.Hour))
	delivered := trimJob(t, root, "bbbbbbbbbbbbbbbb", "ready", now.Add(-30*24*time.Hour))
	idle := trimJob(t, root, "cccccccccccccccc", "ready", now.Add(-20*24*time.Hour))
	manager := &Manager{
		root: root, jobs: map[string]Job{preparing.ID: preparing, delivered.ID: delivered, idle.ID: idle},
		attempts: map[string]*attempt{}, restoring: map[string]Job{}, manifestWork: map[string]*manifestFlight{},
	}
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/download", nil)
	if err := Serve(httptest.NewRecorder(), request, delivered); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(delivered.File)
	if err != nil || time.Since(info.ModTime()) < time.Hour {
		t.Fatalf("delivery changed the media modification time used for range validators: %v, %v", info, err)
	}

	result, err := manager.Trim(1, now)
	if err != nil {
		t.Fatal(err)
	}
	if gone(preparing.File) || gone(delivered.File) || !gone(idle.File) {
		t.Fatalf("preparing gone=%t delivered gone=%t idle gone=%t", gone(preparing.File), gone(delivered.File), gone(idle.File))
	}
	if result.Removed != 1 || result.Bytes != 800 {
		t.Fatalf("result = %#v", result)
	}
}

func TestTrimWithoutCacheIsANoOp(t *testing.T) {
	t.Parallel()
	if result, err := (&Manager{}).Trim(1, time.Now()); err != nil || result != (Trim{}) {
		t.Fatalf("unconfigured trim = %#v, %v", result, err)
	}
}
