package server

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/MikeO7/kinosail/packages/mediaprobe"
	"github.com/MikeO7/kinosail/packages/privatefile"
	"github.com/MikeO7/kinosail/packages/workload"
)

const (
	ocrLanguageMirror    = "https://deb.debian.org/debian/"
	ocrLanguageDataLimit = 64 << 20
	ocrLanguageRetry     = time.Hour
)

var (
	errOCRLanguageUnsupported = errors.New("OCR language data is unavailable for this language")
	errOCRLanguagePackage     = errors.New("OCR language package failed verification")
)

type ocrLanguageStore struct {
	mu       sync.Mutex
	dir      string
	mirror   string
	client   *http.Client
	bsdtar   string
	failures map[string]time.Time
	now      func() time.Time
}

func newOCRLanguageStore(cacheDir string) *ocrLanguageStore {
	client := localIntegrationHTTPClient(2 * time.Minute)
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	return &ocrLanguageStore{dir: filepath.Join(cacheDir, "tessdata"), mirror: ocrLanguageMirror, client: client, bsdtar: "bsdtar", failures: map[string]time.Time{}, now: time.Now}
}

func (store *ocrLanguageStore) supported(code string) bool {
	_, found := ocrLanguagePackages[mediaprobe.OCRLanguage(code)]
	return found
}

func (store *ocrLanguageStore) ensure(ctx context.Context, tesseract, code string) (string, error) {
	name := mediaprobe.OCRLanguage(code)
	if slices.Contains(mediaprobe.InstalledOCRLanguages(ctx, tesseract), name) {
		return "", nil
	}
	pkg, found := ocrLanguagePackages[name]
	if !found {
		return "", errOCRLanguageUnsupported
	}
	path := filepath.Join(store.dir, name+".traineddata")
	store.mu.Lock()
	defer store.mu.Unlock()
	if privatefile.Exists(path, ocrLanguageDataLimit) {
		return store.dir, nil
	}
	if failed, seen := store.failures[name]; seen && store.now().Sub(failed) < ocrLanguageRetry {
		return "", errOCRLanguagePackage
	}
	started := store.now()
	data, err := store.download(ctx, name, pkg)
	if err != nil {
		if ctx.Err() == nil {
			store.failures[name] = store.now()
			slog.WarnContext(ctx, "OCR language data install failed", "operation", "subtitles.ocr_language", "language", name, "failure", ocrLanguageFailure(err))
		}
		return "", err
	}
	if err = privatefile.WriteCache(path, data); err != nil {
		return "", err
	}
	delete(store.failures, name)
	slog.InfoContext(ctx, "OCR language data installed", "operation", "subtitles.ocr_language", "language", name, "bytes", len(data), "duration_ms", store.now().Sub(started).Milliseconds())
	return store.dir, nil
}

func (store *ocrLanguageStore) download(ctx context.Context, name string, pkg ocrLanguagePackage) ([]byte, error) {
	if err := os.MkdirAll(store.dir, 0o700); err != nil {
		return nil, err
	}
	workspace, err := os.MkdirTemp(store.dir, ".download-")
	if err != nil {
		return nil, err
	}
	defer func() { _ = os.RemoveAll(workspace) }()
	archive := filepath.Join(workspace, "language.deb")
	if err = store.fetch(ctx, name, pkg, archive); err != nil {
		return nil, err
	}
	member := "./usr/share/tesseract-ocr/5/tessdata/" + name + ".traineddata"
	for _, arguments := range [][]string{{"-xf", archive, "-C", workspace, "data.tar.xz"}, {"-xf", filepath.Join(workspace, "data.tar.xz"), "-C", workspace, member}} {
		//nolint:gosec // The executable is fixed and the archive matched a pinned Debian SHA-256 before extraction.
		command := exec.CommandContext(ctx, store.bsdtar, arguments...)
		if err = workload.RunContext(ctx, command); err != nil {
			return nil, errors.Join(errOCRLanguagePackage, err)
		}
	}
	file, err := os.Open(filepath.Join(workspace, member))
	if err != nil {
		return nil, errors.Join(errOCRLanguagePackage, err)
	}
	defer func() { _ = file.Close() }()
	data, err := io.ReadAll(io.LimitReader(file, ocrLanguageDataLimit+1))
	if err != nil || len(data) == 0 || len(data) > ocrLanguageDataLimit {
		return nil, errors.Join(errOCRLanguagePackage, err)
	}
	return data, nil
}

func (store *ocrLanguageStore) fetch(ctx context.Context, name string, pkg ocrLanguagePackage, target string) error {
	address := store.mirror + "pool/main/t/tesseract-lang/tesseract-ocr-" + strings.ReplaceAll(name, "_", "-") + "_" + ocrLanguagePackageVersion + "_all.deb"
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, address, nil)
	if err != nil {
		return err
	}
	response, err := store.client.Do(request)
	if err != nil {
		return err
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != http.StatusOK || response.ContentLength > pkg.size {
		return errOCRLanguagePackage
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, pkg.size+1))
	if err != nil {
		return err
	}
	digest := sha256.Sum256(data)
	if int64(len(data)) != pkg.size || hex.EncodeToString(digest[:]) != pkg.sha256 {
		return errOCRLanguagePackage
	}
	return os.WriteFile(target, data, 0o600)
}

func ocrLanguageFailure(err error) string {
	switch {
	case errors.Is(err, errOCRLanguagePackage):
		return "verification"
	case errors.Is(err, context.DeadlineExceeded):
		return "timeout"
	default:
		return "network"
	}
}
