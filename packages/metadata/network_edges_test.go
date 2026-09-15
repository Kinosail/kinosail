package metadata

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"testing"
)

func TestOutboundClientRejectsResolutionAndProhibitedAddresses(t *testing.T) {
	t.Parallel()
	transport := outboundHTTPClient(0, allowedOutboundIP).Transport.(*http.Transport)
	if _, err := transport.DialContext(context.Background(), "tcp", ":80"); err == nil {
		t.Fatal("unresolvable outbound address accepted")
	}
	if _, err := transport.DialContext(context.Background(), "tcp", "127.0.0.1:80"); err == nil {
		t.Fatal("prohibited outbound address accepted")
	}
}

func TestAtomicSaveRejectsFilesystemFailures(t *testing.T) { //nolint:cyclop // Each filesystem stage must return its failure without replacing a target.
	t.Parallel()
	root := t.TempDir()
	parentFile := filepath.Join(root, "parent")
	if err := os.WriteFile(parentFile, []byte("file"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := saveJSON(filepath.Join(parentFile, "value.json"), map[string]bool{"ok": true}); err == nil {
		t.Fatal("directory creation failure was ignored")
	}
	readOnly := filepath.Join(root, "read-only")
	if err := os.Mkdir(readOnly, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(readOnly, 0o700) }) //nolint:gosec // Cleanup restores owner-only directory traversal in the test sandbox.
	if err := saveJSON(filepath.Join(readOnly, "value.json"), map[string]bool{"ok": true}); err == nil {
		t.Fatal("temporary file creation failure was ignored")
	}
	targetDirectory := filepath.Join(root, "target")
	if err := os.Mkdir(targetDirectory, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := saveJSON(targetDirectory, map[string]bool{"ok": true}); err == nil {
		t.Fatal("rename over directory was accepted")
	}
}

func TestConfiguredRequiresCompleteValidProviderSettings(t *testing.T) {
	t.Parallel()
	if !Configured("cache", "token", "https://api.example.com", "https://images.example.com") {
		t.Fatal("complete provider settings rejected")
	}
	if Configured("", "token", "https://api.example.com", "https://images.example.com") || Configured("cache", "", "https://api.example.com", "https://images.example.com") || Configured("cache", "token", "http://example.com", "https://images.example.com") {
		t.Fatal("incomplete or unsafe provider settings accepted")
	}
}
