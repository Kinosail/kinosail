package downloads

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type namedEntry string

func (entry namedEntry) Name() string         { return string(entry) }
func (namedEntry) IsDir() bool                { return false }
func (namedEntry) Type() fs.FileMode          { return 0 }
func (namedEntry) Info() (fs.FileInfo, error) { return nil, os.ErrNotExist }

func TestLoadAndReadJobsReportStorageFailures(t *testing.T) {
	root := t.TempDir()
	failed := validStateJob()
	data, err := json.Marshal(failed)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, failed.ID+".json"), data, 0o600); err != nil {
		t.Fatal(err)
	}
	manager := &Manager{root: root, jobs: make(map[string]Job)}
	if err := manager.load(); err == nil {
		t.Fatal("failed-job recovery succeeded without persistence")
	}
	blocked := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(blocked, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := (&Manager{root: blocked}).readJobs(); err == nil {
		t.Fatal("file cache root was accepted")
	}
}

func TestReadJobCoversConcurrentDeletionAndNonregularState(t *testing.T) {
	manager := &Manager{root: t.TempDir()}
	if _, included, err := manager.readJob(namedEntry("gone.deleting")); err != nil || included {
		t.Fatalf("gone staged deletion = included %v, error %v", included, err)
	}
	if err := os.Mkdir(filepath.Join(manager.root, "directory.json"), 0o700); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(manager.root)
	if err != nil {
		t.Fatal(err)
	}
	if _, included, err := manager.readJob(entries[0]); err == nil || included {
		t.Fatalf("directory state = included %v, error %v", included, err)
	}
	if _, err := manager.loadJob(filepath.Join(manager.root, "missing.json")); !os.IsNotExist(err) {
		t.Fatalf("missing state error = %v", err)
	}
}

func TestStateValidationCoversRemainingBoundaries(t *testing.T) { //nolint:cyclop // One compact boundary matrix covers independent persisted-state constraints.
	entries := make([]os.DirEntry, maximumJobs+1)
	for index := range entries {
		entries[index] = namedEntry("job.json")
	}
	if jobs, err := (&Manager{}).readJobEntries(entries); err == nil || jobs != nil {
		t.Fatalf("oversized state entries = %#v, %v", jobs, err)
	}
	if !tooManyJobEntries(entries) || tooManyJobEntries([]os.DirEntry{namedEntry("note.txt")}) {
		t.Fatal("job entry bound changed")
	}
	if validID("aaaaaaaaaaaaaaag") || !validExtension("") || validExtension(".bad-") {
		t.Fatal("ID or extension validation changed")
	}
	if validOutcome(Job{State: "unknown"}) || validSHA256(strings.Repeat("a", 63)) || validSHA256(strings.Repeat("a", 63)+"g") {
		t.Fatal("outcome or digest validation changed")
	}
}

func validStateJob() Job {
	return Job{
		ID: "aaaaaaaaaaaaaaaa", ItemID: "item", Profile: "viewer", Title: "Film",
		Quality: "original", State: "failed", Error: "interrupted", Extension: ".mp4", Created: time.Now(),
	}
}
