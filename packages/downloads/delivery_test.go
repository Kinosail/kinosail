package downloads

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestServeAddsIntegrityAndRangeHeaders(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "film.mp4")
	content := []byte("0123456789")
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(content)
	job := Job{Title: "Film", File: path, Size: int64(len(content)), SHA256: hex.EncodeToString(digest[:])}
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/download", nil)
	request.Header.Set("Range", "bytes=2-5")
	response := httptest.NewRecorder()
	if err := Serve(response, request, job); err != nil {
		t.Fatal(err)
	}
	if response.Code != http.StatusPartialContent || response.Body.String() != "2345" {
		t.Fatalf("range = %d %q", response.Code, response.Body.String())
	}
	if response.Header().Get("Content-Digest") != "" {
		t.Fatal("unsealed arbitrary range advertised a digest")
	}
	for _, header := range []string{"ETag", "Repr-Digest", "Content-Disposition"} {
		if response.Header().Get(header) == "" {
			t.Errorf("%s header is missing", header)
		}
	}
}

func TestServeHandlesCompletedAndInvalidRanges(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "film.mp4")
	if err := os.WriteFile(path, []byte("0123456789"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		value string
		code  int
	}{
		{"bytes=10-", http.StatusRequestedRangeNotSatisfiable},
		{"bytes=bad", http.StatusRequestedRangeNotSatisfiable},
		{"", http.StatusOK},
	} {
		request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/download", nil)
		request.Header.Set("Range", test.value)
		response := httptest.NewRecorder()
		if err := Serve(response, request, Job{Title: "Film", File: path, Size: 10}); err != nil || response.Code != test.code {
			t.Errorf("Serve(%q) = %d, %v; want %d", test.value, response.Code, err, test.code)
		}
	}
	if err := Serve(httptest.NewRecorder(), httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil), Job{File: "/missing"}); !os.IsNotExist(err) {
		t.Fatalf("missing file error = %v", err)
	}
}

func TestExplicitRangeIsStrictAndBounded(t *testing.T) {
	t.Parallel()
	for value, want := range map[string][2]int64{"bytes=0-7": {0, 8}, "bytes=8-20": {8, 2}} {
		start, length, ok := explicitRange(value, 10)
		if !ok || [2]int64{start, length} != want {
			t.Fatalf("explicitRange(%q) = %d, %d, %v", value, start, length, ok)
		}
	}
	for _, value := range []string{"", "0-1", "bytes=-1", "bytes=0-", "bytes=-1-2", "bytes=8-7", "bytes=10-11", "bytes=0-1,3-4", "bytes=" + strings.Repeat("1", 129) + "-2"} {
		if _, _, ok := explicitRange(value, 10); ok {
			t.Fatalf("invalid range %q was accepted", value)
		}
	}
}

func TestRemovePreservesMemoryWhenStateStagingFails(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()
	root := filepath.Join(directory, "not-a-directory")
	if err := os.WriteFile(root, []byte("blocked"), 0o600); err != nil {
		t.Fatal(err)
	}
	media := filepath.Join(directory, "film.mp4")
	if err := os.WriteFile(media, []byte("media"), 0o600); err != nil {
		t.Fatal(err)
	}
	job := Job{ID: "aaaaaaaaaaaaaaaa", Profile: "viewer", File: media}
	manager := &Manager{root: root, jobs: map[string]Job{job.ID: job}}
	if err := manager.Remove("viewer", job.ID); err == nil {
		t.Fatal("download removal succeeded")
	}
	if _, found := manager.Get("viewer", job.ID); !found {
		t.Fatal("failed removal changed memory")
	}
	if _, err := os.Stat(media); err != nil {
		t.Fatalf("media was not restored: %v", err)
	}
}

func TestWaitHandlesMissingFailureAndCancellation(t *testing.T) {
	t.Parallel()
	manager := &Manager{jobs: map[string]Job{
		"failed":  {ID: "failed", Profile: "viewer", State: "failed", Error: "encode failed"},
		"pending": {ID: "pending", Profile: "viewer", State: "preparing"},
	}}
	if _, err := manager.Wait(t.Context(), "viewer", "missing"); !os.IsNotExist(err) {
		t.Fatalf("missing error = %v", err)
	}
	if _, err := manager.Wait(t.Context(), "viewer", "failed"); err == nil || err.Error() != "encode failed" {
		t.Fatalf("failed error = %v", err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := manager.Wait(ctx, "viewer", "pending"); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation error = %v", err)
	}
}

func TestAccessorsAndPublisherUseCopies(t *testing.T) {
	t.Parallel()
	manager := &Manager{root: "/cache/downloads", jobs: map[string]Job{
		"old": {ID: "old", Profile: "viewer", Created: time.Now().Add(-time.Hour)},
		"new": {ID: "new", Profile: "viewer", Created: time.Now()},
	}}
	if !manager.Configured() || len(manager.jobs) != 2 || len(manager.pending) != 0 || manager.Err() != nil {
		t.Fatalf("manager status is invalid")
	}
	listed := manager.List("viewer")
	if len(listed) != 2 || listed[0].ID != "new" {
		t.Fatalf("list = %#v", listed)
	}
	listed[0].Title = "changed"
	if stored, _ := manager.Get("viewer", "new"); stored.Title == "changed" {
		t.Fatal("list exposed manager storage")
	}
	snapshot := manager.List("viewer")
	if len(snapshot) != 2 {
		t.Fatalf("snapshot = %#v", snapshot)
	}
	published := ""
	manager.SetPublisher(func(job Job) { published = job.ID })
	manager.emit(Job{ID: "event"})
	if published != "event" {
		t.Fatalf("published = %q", published)
	}
}

func TestServeRejectsChangedFileBeforeWritingHeaders(t *testing.T) {
	path := filepath.Join(t.TempDir(), "changed.mp4")
	if err := os.WriteFile(path, []byte("changed"), 0o600); err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/download", nil)
	request.Header.Set("Range", "bytes=0-6")
	if err := Serve(response, request, Job{File: path, Size: 1}); err == nil {
		t.Fatal("changed size accepted")
	}
	if response.Body.Len() != 0 || len(response.Header()) != 0 {
		t.Fatal("rejected asset wrote response")
	}
}
