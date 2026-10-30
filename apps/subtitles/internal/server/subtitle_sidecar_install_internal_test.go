package server

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/MikeO7/kinosail/packages/library"
)

const (
	installedText = "1\n00:00:01,000 --> 00:00:02,000\nInstalled\n"
	currentText   = "1\n00:00:01,000 --> 00:00:02,000\nCurrent\n"
	backupText    = "1\n00:00:01,000 --> 00:00:02,000\nOlder backup\n"
)

func installFixture(t *testing.T) (*subtitleProvider, *subtitleSidecar, string) {
	t.Helper()
	media := t.TempDir()
	item := library.Item{ID: "0123456789abcdef", Kind: "video", Path: filepath.Join(media, "Film.mkv")}
	if err := os.WriteFile(item.Path, []byte("video"), 0o600); err != nil {
		t.Fatal(err)
	}
	provider := &subtitleProvider{index: sidecarTestIndex(item), ledger: openTestSubtitleLedger(t, t.TempDir())}
	target, err := provider.openSidecar(item, "en")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(target.close)
	return provider, target, filepath.Join(media, "Film.en.srt")
}

func installRecord() subtitleRecord {
	now := time.Now().Unix()
	return subtitleRecord{Source: "subdl", Score: 100, ReleaseMatch: 1, CheckedAt: now, InstalledAt: now, Managed: true, Synchronization: "none"}
}

func readText(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(data)
}

func TestInstallSidecarWritesTheFileBackupAndHistoryTogether(t *testing.T) {
	t.Parallel()
	provider, target, path := installFixture(t)
	if err := os.WriteFile(path, []byte(currentText), 0o600); err != nil {
		t.Fatal(err)
	}
	install := sidecarInstall{Key: subtitleRecordKey("0123456789abcdef", "en"), Data: []byte(installedText), Current: []byte(currentText), Backup: true, Record: installRecord(), Action: "updated", Reason: "exact-hash"}
	if err := provider.installSidecar(target, install); err != nil {
		t.Fatal(err)
	}
	record, found, err := provider.ledger.record(install.Key)
	if err != nil || !found || record.Fingerprint != subtitleFingerprint([]byte(installedText)) || readText(t, path) != installedText || readText(t, path+".kinosail.bak") != currentText {
		t.Fatalf("record=%#v found=%t err=%v file=%q backup=%q", record, found, err, readText(t, path), readText(t, path+".kinosail.bak"))
	}
}

func TestInstallSidecarRollsBackANewFileWhenHistoryFails(t *testing.T) {
	t.Parallel()
	provider, target, path := installFixture(t)
	provider.ledger.err = errors.New("ledger unavailable")
	install := sidecarInstall{Key: subtitleRecordKey("0123456789abcdef", "en"), Data: []byte(installedText), Record: installRecord(), Action: "added", Reason: "missing"}
	if err := provider.installSidecar(target, install); err == nil {
		t.Fatal("install succeeded without history")
	}
	if _, err := os.Lstat(path); !os.IsNotExist(err) {
		t.Fatalf("new sidecar kept after failed history: %v", err)
	}
}

func TestInstallSidecarRestoresTheFileAndBackupWhenHistoryFails(t *testing.T) {
	t.Parallel()
	provider, target, path := installFixture(t)
	for name, text := range map[string]string{path: currentText, path + ".kinosail.bak": backupText} {
		if err := os.WriteFile(name, []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	provider.ledger.err = errors.New("ledger unavailable")
	install := sidecarInstall{Key: subtitleRecordKey("0123456789abcdef", "en"), Data: []byte(installedText), Current: []byte(currentText), Backup: true, PreviousBackup: []byte(backupText), Record: installRecord(), Action: "restored", Reason: "restore"}
	if err := provider.installSidecar(target, install); err == nil {
		t.Fatal("install succeeded without history")
	}
	if readText(t, path) != currentText || readText(t, path+".kinosail.bak") != backupText {
		t.Fatalf("after rollback file=%q backup=%q", readText(t, path), readText(t, path+".kinosail.bak"))
	}
}

func TestExclusiveSidecarWriteFallsBackWhenHardLinksAreUnsupported(t *testing.T) {
	t.Parallel()
	_, target, path := installFixture(t)
	if err := target.writeExclusiveDirect("Film.en.srt", []byte(installedText), 0o640); err != nil {
		t.Fatal(err)
	}
	if info, err := os.Stat(path); err != nil || info.Mode().Perm() != 0o640 || readText(t, path) != installedText {
		t.Fatalf("direct exclusive write = %v, %v", info, err)
	}
	if err := target.writeExclusiveDirect("Film.en.srt", []byte(currentText), 0o640); !errors.Is(err, os.ErrExist) || readText(t, path) != installedText {
		t.Fatalf("direct exclusive write replaced an existing file: %v", err)
	}
	for _, unsupported := range []error{syscall.ENOTSUP, &os.LinkError{Op: "link", Err: syscall.ENOTSUP}} {
		if !hardLinkUnsupported(unsupported) {
			t.Fatalf("%v not treated as unsupported", unsupported)
		}
	}
	if hardLinkUnsupported(os.ErrExist) || hardLinkUnsupported(&os.LinkError{Op: "link", Err: os.ErrExist}) {
		t.Fatal("an existing target was treated as missing hard-link support")
	}
}
