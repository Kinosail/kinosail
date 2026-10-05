package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"net/http"
	"net/http/httptest"
	"os"
	"os/signal"
	"path/filepath"
	"regexp"
	"sync"
	"syscall"
	"time"

	"github.com/MikeO7/kinosail-subtitles/internal/server"
)

var (
	fixtureServe = flag.Bool("r06-serve", false, "serve the disposable Save fixture")
	fixtureRoot  = flag.String("r06-root", "", "empty private disposable directory")
	fixtureMode  = flag.String("r06-fault", "", "headers or body")
	itemID       = regexp.MustCompile(`^[a-f0-9]{16}$`)
	operationID  = regexp.MustCompile(`^[a-f0-9]{64}$`)
	applyPath    = regexp.MustCompile(`^/api/v1/subtitle-library/([a-f0-9]{16})/apply$`)
)

const initialSRT = "1\n00:00:01,000 --> 00:00:02,000\nFictional original line\n\n2\n00:00:04,000 --> 00:00:05,000\nFictional later line\n"
const savedSRT = "1\n00:00:01,000 --> 00:00:02,000\nFictional reviewed line\n\n2\n00:00:04,000 --> 00:00:05,000\nFictional reviewed later line\n"

type safeSnapshot struct {
	Protocol                string `json:"protocol"`
	SaveAttempts            int    `json:"saveAttempts"`
	PrepareAttempts         int    `json:"prepareAttempts"`
	ResponseStatus          int    `json:"responseStatus"`
	ActualSaved             bool   `json:"actualSaved"`
	HistoryOnce             bool   `json:"historyOnce"`
	RecoveryMatches         bool   `json:"recoveryMatches"`
	InspectionMatches       bool   `json:"inspectionMatches"`
	ReceiptCompleted        bool   `json:"receiptCompleted"`
	ReceiptSucceeded        bool   `json:"receiptSucceeded"`
	HoldEligible            bool   `json:"holdEligible"`
	HeadersReleased         bool   `json:"headersReleased"`
	BodyReleased            bool   `json:"bodyReleased"`
	ResponseBodyWritten     bool   `json:"responseBodyWritten"`
	ClientCancelled         bool   `json:"clientCancelled"`
	HoldExpired             bool   `json:"holdExpired"`
	ActiveHolds             int    `json:"activeHolds"`
	BrowserCompletedReads   int    `json:"browserCompletedReads"`
	BrowserSavedInspections int    `json:"browserSavedInspections"`
	BoundaryFailed          bool   `json:"boundaryFailed"`
}

type fixture struct {
	tls                      *httptest.Server
	app                      http.Handler
	origin, target, mode     string
	cancel                   context.CancelFunc
	ctx                      context.Context
	mu                       sync.Mutex
	state                    safeSnapshot
	preparedID, preparedItem string
	responseBody             []byte
	responseHeader           http.Header
	eligible, released       chan struct{}
	releaseOnce              sync.Once
	stopOnce                 sync.Once
}

func newFixture(root, mode string) (*fixture, error) {
	if mode != "headers" && mode != "body" || root == "" || !filepath.IsAbs(root) {
		return nil, errors.New("invalid fixture admission")
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 0 {
		return nil, errors.New("fixture directory must be empty")
	}
	for _, name := range []string{"media", "data", "cache"} {
		if os.Mkdir(filepath.Join(root, name), 0700) != nil {
			return nil, errors.New("fixture directory unavailable")
		}
	}
	media := filepath.Join(root, "media", "R06 Fictional Save.mp4")
	target := filepath.Join(root, "media", "R06 Fictional Save.en.srt")
	// This is an indexed name, not playable media. Media endpoints are denied.
	if os.WriteFile(media, []byte("R06 Save-only indexed fixture; no decoded media"), 0600) != nil ||
		os.WriteFile(target, []byte(initialSRT), 0600) != nil {
		return nil, errors.New("fictional fixture unavailable")
	}
	ctx, cancel := context.WithCancel(context.Background())
	f := &fixture{target: target, mode: mode, cancel: cancel, ctx: ctx,
		eligible: make(chan struct{}), released: make(chan struct{}),
		state: safeSnapshot{Protocol: "unreached"}}
	f.tls = httptest.NewUnstartedServer(http.HandlerFunc(f.serve))
	f.origin = "https://" + f.tls.Listener.Addr().String()
	unavailableTool := filepath.Join(root, "unavailable-media-tool")
	f.app = server.New(server.Config{
		Lifecycle: ctx, SubtitleApp: true, RequireAuth: true, AuthURL: f.origin,
		MediaDir: filepath.Join(root, "media"), DataDir: filepath.Join(root, "data"),
		CacheDir: filepath.Join(root, "cache"), FFmpeg: unavailableTool, FFprobe: unavailableTool,
		FPCalc: unavailableTool,
	})
	f.tls.StartTLS()
	return f, nil
}

func (f *fixture) snapshot() safeSnapshot {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.state
}

func (f *fixture) release() { f.releaseOnce.Do(func() { close(f.released) }) }

func (f *fixture) stop() {
	f.stopOnce.Do(func() {
		f.cancel()
		f.release()
		f.tls.CloseClientConnections()
		f.tls.Close()
	})
}

func (f *fixture) actualBody() []byte {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]byte(nil), f.responseBody...)
}

func (f *fixture) actualContentType() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.responseHeader.Get("Content-Type")
}

func runFixture() int {
	if !*fixtureServe {
		return 2
	}
	f, err := newFixture(*fixtureRoot, *fixtureMode)
	if err != nil {
		return 2
	}
	defer f.stop()
	if json.NewEncoder(os.Stdout).Encode(struct {
		Kind, Origin string `json:",omitempty"`
	}{"r06-save-fixture-v1", f.origin}) != nil {
		return 2
	}
	stopping := make(chan os.Signal, 1)
	signal.Notify(stopping, syscall.SIGTERM, syscall.SIGINT)
	defer signal.Stop(stopping)
	select {
	case <-stopping:
	case <-time.After(90 * time.Second):
	}
	return 0
}
