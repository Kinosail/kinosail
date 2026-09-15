package downloads

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

var errInjected = errors.New("injected download file failure")

func TestRemoveReportsMediaStagingFailure(t *testing.T) {
	if err := (&Manager{jobs: map[string]Job{}}).Remove("", "missing"); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("missing empty-profile job error = %v", err)
	}
	job := Job{ID: "aaaaaaaaaaaaaaaa", Profile: "viewer", File: "/media/file"}
	manager := &Manager{root: "/cache", jobs: map[string]Job{job.ID: job}}
	files := deliveryFiles{
		remove: func(string) error { return errInjected },
		rename: os.Rename,
	}
	if err := manager.remove("viewer", job.ID, files); !errors.Is(err, errInjected) {
		t.Fatalf("remove() error = %v", err)
	}
	if _, found := manager.Get("viewer", job.ID); !found {
		t.Fatal("failed media staging changed memory")
	}
}

func TestStageRemovalReportsRenameFailure(t *testing.T) {
	files := deliveryFiles{
		remove: func(string) error { return os.ErrNotExist },
		rename: func(string, string) error { return errInjected },
	}
	if trash, moved, err := stageRemoval("/media/file", files); !errors.Is(err, errInjected) || trash != "" || moved {
		t.Fatalf("stageRemoval() = %q, %v, %v", trash, moved, err)
	}
}

func TestServeReportsStatFailure(t *testing.T) {
	path := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	files := deliveryFiles{open: func(path string) (*os.File, error) {
		file, err := os.Open(path)
		if err == nil {
			err = file.Close()
		}
		return file, err
	}}
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/download", nil)
	if err := serve(httptest.NewRecorder(), request, Job{File: path}, files); err == nil {
		t.Fatal("closed download file was served")
	}
}
