package server

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

func TestOCRLanguageDownloadRejectsUnverifiedPackagesWithoutInstalling(t *testing.T) {
	t.Parallel()
	var requests atomic.Int32
	mirror := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requests.Add(1)
		if request.URL.Path != "/pool/main/t/tesseract-lang/tesseract-ocr-chi-sim_4.1.0-2_all.deb" {
			http.NotFound(writer, request)
			return
		}
		_, _ = writer.Write([]byte("not the pinned Debian package"))
	}))
	defer mirror.Close()
	cache := t.TempDir()
	store := newOCRLanguageStore(cache)
	store.mirror, store.client = mirror.URL+"/", mirror.Client()
	now := time.Unix(1_800_000_000, 0)
	store.now = func() time.Time { return now }
	tesseract := filepath.Join(t.TempDir(), "missing-tesseract")
	for attempt, want := range []int32{1, 1} {
		if _, err := store.ensure(context.Background(), tesseract, "zh-Hans"); !errors.Is(err, errOCRLanguagePackage) || requests.Load() != want {
			t.Fatalf("attempt %d: err = %v, requests = %d", attempt, err, requests.Load())
		}
	}
	if entries, _ := os.ReadDir(filepath.Join(cache, "tessdata")); len(entries) != 0 {
		t.Fatalf("unverified package left files: %v", entries)
	}
	now = now.Add(ocrLanguageRetry)
	if _, err := store.ensure(context.Background(), tesseract, "zh-Hans"); !errors.Is(err, errOCRLanguagePackage) || requests.Load() != 2 {
		t.Fatalf("retry after backoff: err = %v, requests = %d", err, requests.Load())
	}
	if _, err := store.ensure(context.Background(), tesseract, "tlh"); !errors.Is(err, errOCRLanguageUnsupported) || requests.Load() != 2 {
		t.Fatalf("unsupported language: err = %v, requests = %d", err, requests.Load())
	}
}
