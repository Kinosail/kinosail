package backup

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestVersionAndPassphraseBoundariesPrecedeSideEffects(t *testing.T) {
	for _, test := range []struct {
		version string
		valid   bool
	}{
		{version: "dev", valid: true},
		{version: "1.2.3-alpha+build_7", valid: true},
		{version: strings.Repeat("v", 128), valid: true},
		{version: ""},
		{version: ".hidden"},
		{version: "v1/unsafe"},
		{version: "v1\nunsafe"},
		{version: strings.Repeat("v", 129)},
	} {
		if got := validVersion(test.version); got != test.valid {
			t.Fatalf("validVersion(%q) = %v, want %v", test.version, got, test.valid)
		}
	}

	dataDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dataDir, "kinosail.db"), []byte("database"), 0o600); err != nil {
		t.Fatal(err)
	}
	opens := 0
	service := testService(t, func(string) (Database, error) {
		opens++
		return nil, errors.New("database opened")
	}, func([]byte) error { return nil })
	random := &trackedReader{}
	service.random = random
	for name, run := range map[string]func(io.Writer) error{
		"short passphrase": func(output io.Writer) error {
			return service.WriteEncrypted(output, dataDir, strings.Repeat("k", 15), "dev")
		},
		"oversized passphrase": func(output io.Writer) error {
			return service.WriteEncrypted(output, dataDir, strings.Repeat("k", maxPassphraseSize+1), "dev")
		},
		"oversized version": func(output io.Writer) error {
			return service.WriteEncrypted(output, dataDir, strings.Repeat("v", 129), "correct horse battery staple")
		},
	} {
		t.Run(name, func(t *testing.T) {
			output := &countingWriter{}
			if err := run(output); err == nil {
				t.Fatal("invalid boundary was accepted")
			}
			if opens != 0 || random.calls != 0 || output.calls != 0 {
				t.Fatalf("rejected input caused effects: opens=%d random=%d writes=%d", opens, random.calls, output.calls)
			}
		})
	}
}

func TestRestoreValidatesAndAuthenticatesBeforeFilesystemEffects(t *testing.T) {
	service := testService(t, func(string) (Database, error) { return nil, nil }, func([]byte) error { return nil })
	destination := t.TempDir()
	want := map[string]string{
		"settings.json": "{\"name\":\"Before\"}",
		"profiles.json": "[]",
		"kinosail.db":   "database",
	}
	for name, content := range want {
		if err := os.WriteFile(filepath.Join(destination, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	var invalid bytes.Buffer
	contents := map[string][]byte{
		"settings.json": []byte(`{"name":"After"}`),
		"profiles.json": []byte(`{`),
	}
	if err := writeArchive(&invalid, "dev", contents, []string{"settings.json", "profiles.json"}); err != nil {
		t.Fatal(err)
	}
	if err := service.Restore(bytes.NewReader(invalid.Bytes()), destination); err == nil {
		t.Fatal("archive with a late invalid entry was restored")
	}
	assertFilesUnchanged(t, destination, want)

	source := t.TempDir()
	if err := os.WriteFile(filepath.Join(source, "settings.json"), []byte(`{"name":"After"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	var encrypted bytes.Buffer
	if err := service.WriteEncrypted(&encrypted, source, "correct horse battery staple", "dev"); err != nil {
		t.Fatal(err)
	}
	if err := service.RestoreEncrypted(bytes.NewReader(encrypted.Bytes()), destination, "incorrect horse battery staple"); err == nil {
		t.Fatal("encrypted archive authenticated with the wrong passphrase")
	}
	assertFilesUnchanged(t, destination, want)
}

type lifecycleProbe struct {
	acquired, released, writes, verifies atomic.Int64
	active, maximum                      atomic.Int64
}

func (probe *lifecycleProbe) acquire(context.Context) (func(), error) {
	probe.acquired.Add(1)
	return func() { probe.released.Add(1) }, nil
}

func (probe *lifecycleProbe) write(writer io.Writer, _, _ string) error {
	current := probe.active.Add(1)
	for observed := probe.maximum.Load(); current > observed && !probe.maximum.CompareAndSwap(observed, current); observed = probe.maximum.Load() {
	}
	defer probe.active.Add(-1)
	time.Sleep(time.Millisecond)
	probe.writes.Add(1)
	_, err := writer.Write([]byte("encrypted"))
	return err
}

func (probe *lifecycleProbe) verify(reader io.Reader, _ string) error {
	probe.verifies.Add(1)
	data, err := io.ReadAll(reader)
	if err != nil || string(data) != "encrypted" {
		return errors.New("invalid archive")
	}
	return nil
}

func TestManagerSerializesConcurrentLifecycle(t *testing.T) {
	const workers = 8
	probe := &lifecycleProbe{}
	manager := NewManager(ManagerConfig{
		DataDir: t.TempDir(), Directory: t.TempDir(), Key: "test-backup-key", Retention: workers + 1,
		Acquire: probe.acquire, WriteEncrypted: probe.write, VerifyAuto: probe.verify,
	})
	runConcurrentBackups(t, manager, workers)
	assertSerializedLifecycle(t, manager, probe, workers)
}

func runConcurrentBackups(t *testing.T, manager *Manager, workers int) {
	t.Helper()
	start := make(chan struct{})
	errorsSeen := make(chan error, workers)
	var writesDone sync.WaitGroup
	for range workers {
		writesDone.Add(1)
		go func() {
			defer writesDone.Done()
			<-start
			errorsSeen <- manager.Write(t.Context())
		}()
	}
	statusDone := make(chan struct{})
	go func() {
		defer close(statusDone)
		for manager.Status().LastSuccess == "" {
			time.Sleep(time.Millisecond)
		}
	}()
	close(start)
	writesDone.Wait()
	close(errorsSeen)
	<-statusDone
	for err := range errorsSeen {
		if err != nil {
			t.Fatal(err)
		}
	}
}

func assertSerializedLifecycle(t *testing.T, manager *Manager, probe *lifecycleProbe, workers int) {
	t.Helper()
	want := int64(workers)
	if probe.acquired.Load() != want || probe.released.Load() != want || probe.writes.Load() != want || probe.verifies.Load() != want {
		t.Fatalf("lifecycle counts: acquire=%d release=%d write=%d verify=%d", probe.acquired.Load(), probe.released.Load(), probe.writes.Load(), probe.verifies.Load())
	}
	if probe.maximum.Load() != 1 {
		t.Fatalf("concurrent archive writes = %d", probe.maximum.Load())
	}
	if files, err := manager.Files(); err != nil || len(files) != workers {
		t.Fatalf("backup files = %v, %v", files, err)
	}
}

func TestManagerReleasesCapacityAndCleansTemporaryFileAfterFailure(t *testing.T) {
	want := errors.New("archive failed")
	var released, verified atomic.Int64
	directory := t.TempDir()
	manager := NewManager(ManagerConfig{
		DataDir: t.TempDir(), Directory: directory, Key: "test-backup-key",
		Acquire: func(context.Context) (func(), error) {
			return func() { released.Add(1) }, nil
		},
		WriteEncrypted: func(io.Writer, string, string) error { return want },
		VerifyAuto: func(io.Reader, string) error {
			verified.Add(1)
			return nil
		},
	})
	if err := manager.Write(t.Context()); !errors.Is(err, want) {
		t.Fatalf("write error = %v", err)
	}
	if released.Load() != 1 || verified.Load() != 0 {
		t.Fatalf("release=%d verify=%d", released.Load(), verified.Load())
	}
	entries, err := os.ReadDir(directory)
	if err != nil || len(entries) != 0 {
		t.Fatalf("temporary backup remains: %v, %v", entries, err)
	}
}

func TestCommandRejectionsHaveNoCallbacksOrIO(t *testing.T) {
	service := testService(t, func(string) (Database, error) { return nil, errors.New("database opened") }, func([]byte) error { return nil })
	versionCalls := 0
	command := BindCommand(service, func() string {
		versionCalls++
		return "dev"
	})
	blocked := &trackedReadWriter{}
	for _, test := range []struct {
		args    []string
		input   io.Reader
		output  io.Writer
		handled bool
		err     bool
	}{
		{args: []string{"version"}, input: blocked, output: blocked},
		{args: []string{"backup", "extra"}, input: blocked, output: blocked, handled: true, err: true},
		{args: []string{"restore", "extra"}, input: blocked, output: blocked, handled: true, err: true},
		{args: []string{"backup"}, input: blocked, handled: true, err: true},
		{args: []string{"backup", "verify"}, output: blocked, handled: true, err: true},
		{args: []string{"restore"}, output: blocked, handled: true, err: true},
	} {
		handled, err := command(test.args, test.input, test.output, t.TempDir(), "")
		if handled != test.handled || (err != nil) != test.err {
			t.Fatalf("command %v = handled %v, error %v", test.args, handled, err)
		}
	}
	if versionCalls != 0 || blocked.reads != 0 || blocked.writes != 0 {
		t.Fatalf("rejected commands caused effects: version=%d reads=%d writes=%d", versionCalls, blocked.reads, blocked.writes)
	}
}

func assertFilesUnchanged(t *testing.T, directory string, want map[string]string) {
	t.Helper()
	for name, content := range want {
		data, err := os.ReadFile(filepath.Join(directory, name))
		if err != nil || string(data) != content {
			t.Fatalf("%s changed to %q: %v", name, data, err)
		}
	}
	entries, err := os.ReadDir(directory)
	if err != nil || len(entries) != len(want) {
		t.Fatalf("restore left filesystem effects: %v, %v", entries, err)
	}
}

type trackedReader struct{ calls int }

func (reader *trackedReader) Read([]byte) (int, error) {
	reader.calls++
	return 0, errors.New("unexpected read")
}

type trackedReadWriter struct{ reads, writes int }

func (value *trackedReadWriter) Read([]byte) (int, error) {
	value.reads++
	return 0, errors.New("unexpected read")
}

func (value *trackedReadWriter) Write(data []byte) (int, error) {
	value.writes++
	return len(data), errors.New("unexpected write")
}
