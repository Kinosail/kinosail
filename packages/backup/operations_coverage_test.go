package backup

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestBackupCoverageClosesManagerFailureEdges(t *testing.T) { //nolint:cyclop,funlen // One matrix covers scheduler and filesystem failures.
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	_ = NewManager(ManagerConfig{Context: ctx, DataDir: "/data", Directory: "/backups", Key: "key", Retention: 1, Interval: time.Millisecond, WriteEncrypted: func(io.Writer, string, string) error { return nil }, VerifyAuto: func(io.Reader, string) error { return nil }})

	invalid := NewManager(ManagerConfig{})
	if err := invalid.WriteNow(); err == nil {
		t.Fatal("invalid manager wrote immediately")
	}
	if err := invalid.writeLocked(); err == nil {
		t.Fatal("invalid manager wrote while locked")
	}
	missingImplementation := NewManager(ManagerConfig{DataDir: "/data", Directory: "/backups", Key: "key"})
	if err := missingImplementation.WriteNow(); err == nil {
		t.Fatal("missing backup implementation was accepted")
	}

	file := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(file, []byte("file"), 0o600); err != nil {
		t.Fatal(err)
	}
	mkdirFailure := testManager(t.TempDir(), filepath.Join(file, "child"))
	if err := mkdirFailure.writeLocked(); err == nil {
		t.Fatal("backup directory failure was accepted")
	}

	listFailure := testManager(t.TempDir(), t.TempDir())
	listFailure.readDirectory = func(string) ([]os.DirEntry, error) { return nil, errors.New("list failed") }
	if err := listFailure.writeLocked(); err == nil {
		t.Fatal("post-write list failure was discarded")
	}
	if status := listFailure.Status(); status.LastError == "" {
		t.Fatal("status discarded list failure")
	}
	if err := listFailure.verifyLocked(); err == nil {
		t.Fatal("verification discarded list failure")
	}
	if _, err := listFailure.Files(); err == nil {
		t.Fatal("file discovery discarded list failure")
	}

	infoFailure := testManager(t.TempDir(), t.TempDir())
	infoFailure.readDirectory = func(string) ([]os.DirEntry, error) {
		return []os.DirEntry{failingDirectoryEntry{name: "kinosail-test.backup"}}, nil
	}
	if _, err := infoFailure.Files(); err == nil {
		t.Fatal("entry metadata failure was discarded")
	}

	retention := testManager(t.TempDir(), t.TempDir())
	retention.retention = 0
	old := filepath.Join(retention.directory, "kinosail-old.backup")
	if err := os.WriteFile(old, []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	retention.remove = func(string) error { return errors.New("remove failed") }
	if err := retention.writeLocked(); err == nil {
		t.Fatal("retention failure was discarded")
	}
	if err := os.WriteFile(filepath.Join(retention.directory, "unrelated"), []byte("skip"), 0o600); err != nil {
		t.Fatal(err)
	}
	retention.remove = os.Remove
	retention.retention = 7
	if _, err := retention.Files(); err != nil {
		t.Fatal(err)
	}
}

func TestBackupCoverageClosesRestoreFailureEdges(t *testing.T) { //nolint:cyclop,funlen,gocognit // One matrix covers each atomic restore failure.
	base := testService(t, func(string) (Database, error) { return nil, nil }, func([]byte) error { return nil })
	for name, file := range map[string]*restoreFailureFile{
		"write": {writeErr: errors.New("write failed")},
		"sync":  {syncErr: errors.New("sync failed")},
		"close": {closeErr: errors.New("close failed")},
	} {
		t.Run(name, func(t *testing.T) {
			service := *base
			service.restore.createTemp = func(string, string) (restoreFile, error) {
				file.path = filepath.Join(t.TempDir(), "temporary")
				return file, nil
			}
			if _, err := service.stageRestoreFiles(t.TempDir(), map[string][]byte{"settings.json": []byte(`{}`)}); err == nil {
				t.Fatal("restore staging failure was discarded")
			}
		})
	}
	createFailure := *base
	createFailure.restore.createTemp = func(string, string) (restoreFile, error) { return nil, errors.New("create failed") }
	if _, err := createFailure.stageRestoreFiles(t.TempDir(), map[string][]byte{"settings.json": []byte(`{}`)}); err == nil {
		t.Fatal("restore create failure was discarded")
	}
	if err := createFailure.save(map[string][]byte{"settings.json": []byte(`{}`)}, t.TempDir(), []string{"settings.json"}); err == nil {
		t.Fatal("save staging failure was discarded")
	}

	moveFailure := *base
	checks := 0
	moveFailure.restore.lstat = func(string) (os.FileInfo, error) {
		checks++
		if checks <= 4 {
			return nil, os.ErrNotExist
		}
		return nil, errors.New("lstat failed")
	}
	if err := moveFailure.save(map[string][]byte{"settings.json": []byte(`{}`)}, t.TempDir(), []string{"settings.json"}); err == nil {
		t.Fatal("save move failure was discarded")
	}

	installFailure := *base
	installFailure.restore.rename = func(string, string) error { return errors.New("rename failed") }
	if err := installFailure.save(map[string][]byte{"settings.json": []byte(`{}`)}, t.TempDir(), []string{"settings.json"}); err == nil {
		t.Fatal("save install failure was discarded")
	}
	if _, err := installFailure.installRestoreFiles(t.TempDir(), map[string]string{"settings.json": "missing"}); err == nil {
		t.Fatal("install rename failure was discarded")
	}

	vacant := *base
	vacant.restore.createTemp = func(string, string) (restoreFile, error) { return nil, errors.New("create failed") }
	if _, err := vacant.vacantRestorePath(t.TempDir()); err == nil {
		t.Fatal("vacant path create failure was discarded")
	}
	vacant.restore.createTemp = func(string, string) (restoreFile, error) {
		return &restoreFailureFile{path: "temporary", closeErr: errors.New("close failed")}, nil
	}
	if _, err := vacant.vacantRestorePath(t.TempDir()); err == nil {
		t.Fatal("vacant path close failure was discarded")
	}
	vacant.restore.createTemp = func(string, string) (restoreFile, error) { return &restoreFailureFile{path: "temporary"}, nil }
	vacant.restore.remove = func(string) error { return errors.New("remove failed") }
	if _, err := vacant.vacantRestorePath(t.TempDir()); err == nil {
		t.Fatal("vacant path remove failure was discarded")
	}

	original := filepath.Join(t.TempDir(), "settings.json")
	if err := os.WriteFile(original, []byte(`{}`), 0o600); err != nil {
		t.Fatal(err)
	}
	move := *base
	move.restore.createTemp = func(string, string) (restoreFile, error) { return nil, errors.New("vacant failed") }
	if _, err := move.moveRestoreTargets(filepath.Dir(original), []string{"settings.json"}); err == nil {
		t.Fatal("vacant target failure was discarded")
	}
	move.restore.createTemp = defaultRestoreOperations().createTemp
	move.restore.rename = func(string, string) error { return errors.New("rename failed") }
	if _, err := move.moveRestoreTargets(filepath.Dir(original), []string{"settings.json"}); err == nil {
		t.Fatal("move rename failure was discarded")
	}
}

func TestMustNewPanicsForInvalidStaticConfiguration(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("MustNew accepted invalid configuration")
		}
	}()
	MustNew(Config{})
}

type restoreFailureFile struct {
	path                        string
	writeErr, syncErr, closeErr error
}

func (file *restoreFailureFile) Name() string                   { return file.path }
func (file *restoreFailureFile) Write(data []byte) (int, error) { return len(data), file.writeErr }
func (file *restoreFailureFile) Sync() error                    { return file.syncErr }
func (file *restoreFailureFile) Close() error                   { return file.closeErr }

type failingDirectoryEntry struct{ name string }

func (entry failingDirectoryEntry) Name() string         { return entry.name }
func (failingDirectoryEntry) IsDir() bool                { return false }
func (failingDirectoryEntry) Type() os.FileMode          { return 0 }
func (failingDirectoryEntry) Info() (os.FileInfo, error) { return nil, errors.New("info failed") }
