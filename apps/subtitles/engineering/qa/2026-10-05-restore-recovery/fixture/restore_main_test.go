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
	tool := filepath.Join(directory, "unavailable-media-tool")
	f.app = server.New(server.Config{
		Lifecycle: ctx, SubtitleApp: true, RequireAuth: true, AuthURL: target.origin,
		MediaDir: filepath.Join(directory, "media"), DataDir: filepath.Join(directory, "data"),
		CacheDir: filepath.Join(directory, "cache"), FFmpeg: tool, FFprobe: tool, FPCalc: tool,
	})
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
