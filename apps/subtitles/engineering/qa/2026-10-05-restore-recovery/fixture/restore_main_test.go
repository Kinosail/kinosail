package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/MikeO7/kinosail-subtitles/internal/server"
)

var (
	restoreServe = flag.Bool("r06-restore-serve", false, "serve the disposable Restore fixture")
	restoreRoot  = flag.String("r06-restore-root", "", "empty private disposable directory")
	restoreMode  = flag.String("r06-restore-fault", "", "headers or inspect-body")
)

type restoreSnapshot struct {
	Protocol                     string `json:"protocol"`
	SetupSaveAttempts            int    `json:"setupSaveAttempts"`
	RestoreAttempts              int    `json:"restoreAttempts"`
	PrepareAttempts              int    `json:"prepareAttempts"`
	ResponseStatus               int    `json:"responseStatus"`
	InspectionResponseStatus     int    `json:"inspectionResponseStatus"`
	ActiveHolds                  int    `json:"activeHolds"`
	BrowserCompletedReceiptReads int    `json:"browserCompletedReceiptReads"`
	BrowserRestoredInspections   int    `json:"browserRestoredInspections"`
	ActualRestored               bool   `json:"actualRestored"`
	HistoryOnce                  bool   `json:"historyOnce"`
	RecoverySwapped              bool   `json:"recoverySwapped"`
	InspectionMatches            bool   `json:"inspectionMatches"`
	ReceiptCompleted             bool   `json:"receiptCompleted"`
	ReceiptSucceeded             bool   `json:"receiptSucceeded"`
	HoldEligible                 bool   `json:"holdEligible"`
	HeadersReleased              bool   `json:"headersReleased"`
	BodyReleased                 bool   `json:"bodyReleased"`
	RestoreResponseDelivered     bool   `json:"restoreResponseDelivered"`
	InspectionResponseDelivered  bool   `json:"inspectionResponseDelivered"`
	RestoreClientCancelled       bool   `json:"restoreClientCancelled"`
	InspectionClientCancelled    bool   `json:"inspectionClientCancelled"`
	HoldExpired                  bool   `json:"holdExpired"`
	BoundaryFailed               bool   `json:"boundaryFailed"`
}

type restoreRig struct {
	target                              *restoreTarget
	app                                 http.Handler
	files                               *os.Root
	ctx                                 context.Context
	cancel                              context.CancelFunc
	mode, item                          string
	currentBefore                       []byte
	mu                                  sync.Mutex
	owned                               sync.WaitGroup
	state                               restoreSnapshot
	restoreHeaders, inspectionHeaders   http.Header
	restoreBody, inspectionBody         []byte
	eligible, released                  chan struct{}
	releaseOnce, eligibleOnce, stopOnce sync.Once
	inspectionHeld, cleanupFailed       bool
}

func TestMain(m *testing.M) {
	flag.Parse()
	if *restoreServe {
		os.Exit(runRestoreFixture())
	}
	os.Exit(m.Run())
}

func newRestoreRig(target *restoreTarget, directory, mode string) (*restoreRig, error) {
	if directory == "" || !filepath.IsAbs(directory) {
		return nil, errors.New("Restore fixture root unavailable")
	}
	if mode != "none" && mode != "headers" && mode != "inspect-body" {
		return nil, errors.New("Restore fixture fault unavailable")
	}
	files, err := prepareRestoreFiles(directory)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithCancel(context.Background())
	f := &restoreRig{
		target: target, files: files, ctx: ctx, cancel: cancel, mode: mode,
		eligible: make(chan struct{}), released: make(chan struct{}), state: restoreSnapshot{Protocol: "unreached"},
	}
	actualConfig, err := restoreApplicationConfig(ctx, directory, target.origin)
	if err != nil {
		cancel()
		return nil, errors.Join(err, files.Close())
	}
	f.app = server.New(actualConfig)
	target.tls.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { f.serve(target, w, r) })
	target.tls.StartTLS()
	return f, nil
}

func (f *restoreRig) failBoundary()             { f.mu.Lock(); f.state.BoundaryFailed = true; f.mu.Unlock() }
func (f *restoreRig) snapshot() restoreSnapshot { f.mu.Lock(); defer f.mu.Unlock(); return f.state }
func (f *restoreRig) release()                  { f.releaseOnce.Do(func() { close(f.released) }) }
func (f *restoreRig) stop() bool {
	f.stopOnce.Do(func() {
		f.cancel()
		f.release()
		networkOK := f.target.stop()
		f.owned.Wait()
		if err := f.files.Close(); err != nil || !networkOK {
			f.mu.Lock()
			f.state.BoundaryFailed, f.cleanupFailed = true, true
			f.mu.Unlock()
		}
	})
	f.mu.Lock()
	defer f.mu.Unlock()
	return !f.cleanupFailed
}

func runRestoreFixture() (exit int) {
	target, err := newOwnedRestoreTarget()
	if err != nil {
		return 2
	}
	defer func() {
		if !target.stop() {
			exit = 2
		}
	}()
	f, err := newRestoreRig(target, *restoreRoot, *restoreMode)
	if err != nil {
		return 2
	}
	defer func() {
		if !f.stop() {
			exit = 2
		}
	}()
	if json.NewEncoder(os.Stdout).Encode(struct{ Kind, Origin string }{"r06-restore-fixture-v1", target.origin}) != nil {
		return 2
	}
	stopping := make(chan os.Signal, 1)
	signal.Notify(stopping, syscall.SIGTERM, syscall.SIGINT)
	defer signal.Stop(stopping)
	select {
	case <-stopping:
	case <-time.After(120 * time.Second):
	}
	return 0
}

const restoreHardwareSentinel = "unavailable-hardware-devices"

func restoreApplicationConfig(ctx context.Context, directory, origin string) (server.Config, error) {
	devices, err := restoreHardwareTargets(directory)
	if err != nil {
		return server.Config{}, err
	}
	tool := filepath.Join(directory, "unavailable-media-tool")
	return server.Config{
		Lifecycle: ctx, SubtitleApp: true, RequireAuth: true, AuthURL: origin,
		MediaDir: filepath.Join(directory, "media"), DataDir: filepath.Join(directory, "data"),
		CacheDir: filepath.Join(directory, "cache"), FFmpeg: tool, FFprobe: tool, FPCalc: tool,
		HardwareDevices: devices, ProbeHardware: false, DLNAURL: "",
	}, nil
}

func restoreHardwareTargets(directory string) ([]string, error) {
	root, err := os.OpenRoot(directory)
	if err != nil {
		return nil, errors.New("Restore hardware root unavailable")
	}
	_, statErr := root.Lstat(restoreHardwareSentinel)
	closeErr := root.Close()
	if !errors.Is(statErr, os.ErrNotExist) || closeErr != nil {
		return nil, errors.New("Restore hardware target must be absent")
	}
	return []string{filepath.Join(directory, restoreHardwareSentinel)}, nil
}

// These isolated controls cover startup accesses that the Restore journey's
// admitted public routes cannot observe. They never invoke the application.
func TestRestoreStartupUsesOnlyOwnedHardwareTarget(t *testing.T) {
	directory := newRestoreStartupDirectory(t)
	configured, err := restoreApplicationConfig(context.Background(), directory, "https://127.0.0.1:1")
	if err != nil {
		t.Fatal("Restore startup configuration unavailable")
	}
	if len(configured.HardwareDevices) != 1 {
		t.Fatal("Restore startup must avoid default host hardware discovery")
	}
	want := filepath.Join(directory, restoreHardwareSentinel)
	if configured.HardwareDevices[0] != want || configured.ProbeHardware || configured.DLNAURL != "" {
		t.Fatal("Restore startup hardware or discovery isolation changed")
	}
	if _, err = os.Lstat(want); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("Restore startup hardware target must remain nonexistent")
	}
	if configured.DataDir != filepath.Join(directory, "data") || !configured.RequireAuth || configured.AuthURL != "https://127.0.0.1:1" {
		t.Fatal("Restore startup must preserve owned state and authentication")
	}
}

func TestRestoreStartupRejectsExistingHardwareTargets(t *testing.T) {
	cases := []struct {
		name   string
		create func(string) error
	}{
		{"file", func(path string) error { return os.WriteFile(path, []byte("owned sentinel"), 0o600) }},
		{"directory", func(path string) error { return os.Mkdir(path, 0o700) }},
		{"dangling-symlink", func(path string) error { return os.Symlink("owned-missing-target", path) }},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			directory := newRestoreStartupDirectory(t)
			path := filepath.Join(directory, restoreHardwareSentinel)
			if err := testCase.create(path); err != nil {
				t.Fatal("Restore hardware rejection fixture unavailable")
			}
			before, err := os.Lstat(path)
			if err != nil {
				t.Fatal("Restore hardware rejection witness unavailable")
			}
			configured, err := restoreApplicationConfig(context.Background(), directory, "https://127.0.0.1:1")
			if err == nil || len(configured.HardwareDevices) != 0 {
				t.Fatal("Restore must reject an existing hardware target before application startup")
			}
			assertRestoreHardwareWitness(t, path, before)
		})
	}
}

func assertRestoreHardwareWitness(t *testing.T, path string, before os.FileInfo) {
	t.Helper()
	after, err := os.Lstat(path)
	if err != nil || !os.SameFile(before, after) || before.Mode() != after.Mode() || before.Size() != after.Size() {
		t.Fatal("Restore hardware rejection must preserve its owned witness")
	}
	if before.Mode().IsRegular() {
		contents, readErr := os.ReadFile(path)
		if readErr != nil || string(contents) != "owned sentinel" {
			t.Fatal("Restore hardware rejection changed owned bytes")
		}
	}
}
