package backup

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestManagerWritesVerifiesAndReportsEncryptedBackup(t *testing.T) { //nolint:cyclop // One test verifies the public backup lifecycle.
	t.Parallel()
	directory := t.TempDir()
	manager := testManager(t.TempDir(), directory)
	if err := manager.WriteNow(); err != nil {
		t.Fatal(err)
	}
	files, err := manager.Files()
	if err != nil || len(files) != 1 {
		t.Fatalf("files = %v, %v", files, err)
	}
	status := manager.Status()
	if !status.Enabled || !status.Encrypted || status.Latest == "" || status.LastSuccess == "" || status.LastVerified == "" || status.LastError != "" {
		t.Fatalf("status = %#v", status)
	}
	if err := manager.Verify(); err != nil {
		t.Fatal(err)
	}
}

func TestManagerRejectsInvalidConfigurationBeforeWaiting(t *testing.T) {
	t.Parallel()
	acquired := false
	manager := NewManager(ManagerConfig{Acquire: func(context.Context) (func(), error) {
		acquired = true
		return func() {}, nil
	}})
	if err := manager.Write(t.Context()); err == nil || !strings.Contains(err.Error(), "not configured") || acquired {
		t.Fatalf("write = %v, acquired = %v", err, acquired)
	}
	if manager.Status().LastError == "" {
		t.Fatal("invalid configuration was not reported")
	}
}

func TestManagerUsesCapacityAndPropagatesArchiveFailures(t *testing.T) {
	t.Parallel()
	want := errors.New("capacity unavailable")
	manager := testManager(t.TempDir(), t.TempDir())
	manager.acquire = func(context.Context) (func(), error) { return nil, want }
	if err := manager.Write(t.Context()); !errors.Is(err, want) {
		t.Fatalf("capacity error = %v", err)
	}
	manager.acquire = nil
	manager.writeEncrypted = func(io.Writer, string, string) error { return errors.New("archive failed") }
	if err := manager.Write(t.Context()); err == nil || !strings.Contains(err.Error(), "archive failed") {
		t.Fatalf("archive error = %v", err)
	}
}

func TestManagerRetentionAndDiscoveryIgnoreNonFiles(t *testing.T) {
	t.Parallel()
	directory := filepath.Join(t.TempDir(), "backups[home]")
	if err := os.Mkdir(directory, 0o700); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"kinosail-20260829T120000Z.backup", "kinosail-20260829T120001Z.tar.gz"} {
		if err := os.WriteFile(filepath.Join(directory, name), []byte("archive"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Mkdir(filepath.Join(directory, "kinosail-fake.backup"), 0o700); err != nil {
		t.Fatal(err)
	}
	manager := testManager(t.TempDir(), directory)
	manager.retention = 1
	if err := manager.WriteNow(); err != nil {
		t.Fatal(err)
	}
	files, err := manager.Files()
	if err != nil || len(files) != 1 || !strings.HasSuffix(files[0], ".backup") {
		t.Fatalf("retained files = %v, %v", files, err)
	}
}

func TestManagerVerificationFailuresAndRetryPolicy(t *testing.T) {
	t.Parallel()
	manager := NewManager(ManagerConfig{DataDir: t.TempDir(), Directory: t.TempDir(), Interval: 24 * time.Hour})
	if manager.retryDelay() != 24*time.Hour {
		t.Fatalf("invalid retry = %v", manager.retryDelay())
	}
	manager.key = "configured-key"
	manager.writeEncrypted = func(io.Writer, string, string) error { return nil }
	manager.verifyAuto = func(io.Reader, string) error { return errors.New("corrupt") }
	if manager.retryDelay() != time.Minute {
		t.Fatalf("transient retry = %v", manager.retryDelay())
	}
	if err := manager.Verify(); err == nil || !strings.Contains(err.Error(), "no backup") {
		t.Fatalf("empty verification = %v", err)
	}
	path := filepath.Join(manager.directory, "kinosail-test.backup")
	if err := os.WriteFile(path, []byte("archive"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := manager.Verify(); err == nil || manager.Status().LastError == "" {
		t.Fatalf("verification = %v, status = %#v", err, manager.Status())
	}
}

func TestManagerScheduleStopsAfterSuccessfulAndFailedAttempts(t *testing.T) {
	t.Parallel()
	for _, fail := range []bool{false, true} {
		ctx, cancel := context.WithCancel(t.Context())
		manager := testManager(t.TempDir(), t.TempDir())
		manager.interval = time.Millisecond
		manager.acquire = func(context.Context) (func(), error) {
			if fail {
				cancel()
				return nil, errors.New("busy")
			}
			return func() {}, nil
		}
		if !fail {
			write := manager.writeEncrypted
			manager.writeEncrypted = func(writer io.Writer, dataDir, key string) error {
				defer cancel()
				return write(writer, dataDir, key)
			}
		}
		manager.schedule(ctx)
		if !fail {
			if files, err := manager.Files(); err != nil || len(files) != 1 {
				t.Fatalf("scheduled files = %v, %v", files, err)
			}
		}
	}
}

func testManager(dataDir, directory string) *Manager {
	return NewAutomaticManager(
		context.Background(), dataDir, directory, "test-backup-key", 0, 7, nil,
		func(writer io.Writer, _, _ string) error {
			_, err := writer.Write([]byte("encrypted"))
			return err
		},
		func(reader io.Reader, _ string) error {
			data, err := io.ReadAll(reader)
			if err != nil || !bytes.Equal(data, []byte("encrypted")) {
				return errors.New("invalid archive")
			}
			return nil
		},
	)
}
