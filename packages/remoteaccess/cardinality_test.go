package remoteaccess_test

import (
	"os"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail/packages/remoteaccess"
)

func TestNewRejectsMultipleDependencySetsBeforeFiles(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()
	config := remoteaccess.Config{Enabled: true, PublicHTTPS: true, Domain: "family", Token: strings.Repeat("a", 32), Listen: "127.0.0.1:8443", DataDir: directory}
	if _, err := remoteaccess.New(config, remoteaccess.Dependencies{}, remoteaccess.Dependencies{}); err == nil {
		t.Fatal("multiple dependency sets accepted")
	}
	entries, err := os.ReadDir(directory)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("rejected dependencies created %d file entries", len(entries))
	}
}
