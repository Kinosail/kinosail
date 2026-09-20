package downloads

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestOriginalDownloadCopyIsPrivateAndRejectsInvalidSource(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	source, output := filepath.Join(root, "source"), filepath.Join(root, "copy")
	if err := os.WriteFile(source, []byte("original media"), 0o600); err != nil {
		t.Fatal(err)
	}
	manager := New(Config{Context: t.Context(), Cache: t.TempDir()})
	if err := manager.copyOriginal(t.Context(), source, output); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(output)
	if err != nil || string(data) != "original media" {
		t.Fatalf("copied bytes=%q %v", data, err)
	}
	info, err := os.Stat(output)
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("copy permissions=%v %v", info, err)
	}
	assertInvalidOriginalCopies(t, root, source, output)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if err := copyFileContext(ctx, source, output); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled copy=%v", err)
	}
	if err := manager.copyOriginal(ctx, source, output); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled original=%v", err)
	}
}

func assertInvalidOriginalCopies(t *testing.T, root, source, output string) {
	t.Helper()
	for _, paths := range [][2]string{{filepath.Join(root, "missing"), output}, {root, output}, {source, root}, {source, filepath.Join(root, "absent", "copy")}} {
		if err := copyFileContext(t.Context(), paths[0], paths[1]); err == nil {
			t.Fatalf("invalid copy %v accepted", paths)
		}
	}
}
