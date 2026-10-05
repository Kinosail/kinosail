package main

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/MikeO7/kinosail-subtitles/internal/server"
)

type r16Fixture struct {
	target *r16Target
	files *r16OwnedFiles
	peer *r16ProviderPeer
	cancel context.CancelFunc
	mu sync.Mutex
	stopOnce sync.Once
	stopped bool
}

func newR16PublicFixture(t *testing.T, providerEnabled bool) r16PublicFixture {
	t.Helper()
	lifecycle, cancel := context.WithTimeout(t.Context(), 150*time.Second)
	target, err := r16NewTarget(lifecycle)
	if err != nil {
		cancel()
		t.Fatal("owned R16 TLS target could not be allocated")
	}
	f := &r16Fixture{target: target, cancel: cancel}
	t.Cleanup(func() {
		ctx, stop := context.WithTimeout(context.WithoutCancel(t.Context()), 5*time.Second)
		defer stop()
		if !f.StopAndJoin(ctx) {
			t.Error("owned R16 fixture cleanup did not settle")
		}
	})
	if !f.initialize(t, lifecycle, providerEnabled) {
		t.Fatal("owned R16 public fixture prerequisites were not established")
	}
	return f
}

func (f *r16Fixture) initialize(t *testing.T, lifecycle context.Context, providerEnabled bool) bool {
	t.Helper()
	files, err := r16CreateFiles(providerEnabled)
	if err != nil {
		return false
	}
	f.files = files
	subtitles := server.SubtitleConfig{}
	if providerEnabled {
		peer, peerErr := r16NewProviderPeer(lifecycle)
		if peerErr != nil {
			return false
		}
		f.peer = peer
		subtitles.URL, subtitles.APIKey = peer.origin, peer.key
	}
	config := server.Config{
		Lifecycle: lifecycle, SubtitleApp: true,
		MediaDir: files.mediaPath, DataDir: files.statePath,
		CacheDir: files.cachePath, RequireAuth: true,
		AuthURL: f.target.origin, FFprobe: "",
		ProbeHardware: false, HardwareDevices: []string{files.unavailableDevice},
		DLNAURL: "",
		FFmpeg: files.unavailableTool, FPCalc: files.unavailableTool,
		Subtitles: subtitles,
	}
	f.target.start(server.New(config))
	setup, cancel := context.WithTimeout(t.Context(), 20*time.Second)
	defer cancel()
	if r16EnrollOwner(t, setup, f.target) != nil {
		f.target.fail()
		return false
	}
	return r16RegisterActualCatalogue(setup, f.target, providerEnabled) == nil
}

func (f *r16Fixture) ItemID() string {
	return f.target.registeredItem()
}

func (f *r16Fixture) Request(ctx context.Context, method, route string, body []byte, authority r16Authority) r16Response {
	return f.target.request(ctx, method, route, body, authority)
}

func (f *r16Fixture) OpenEvents(ctx context.Context, cursor string, headerBudget time.Duration) r16EventStream {
	return f.target.openEvents(ctx, cursor, headerBudget)
}

func (f *r16Fixture) Files(t *testing.T) r16Files {
	t.Helper()
	files, err := f.files.snapshot()
	if err != nil {
		f.target.fail()
		t.Fatal("owned R16 file witness could not be read safely")
	}
	return files
}

func (f *r16Fixture) ProviderCalls() int {
	if f.peer == nil {
		return 0
	}
	return f.peer.calls()
}

func (f *r16Fixture) StopAndJoin(ctx context.Context) bool {
	f.stopOnce.Do(func() {
		f.cancel()
		network := f.target.stop(ctx)
		provider := f.peer == nil || f.peer.stop(ctx)
		settled := f.target.settled() && (f.peer == nil || f.peer.settled())
		if !settled {
			return
		}
		files := f.files == nil || f.files.close()
		f.mu.Lock()
		f.stopped = network && provider && files
		f.mu.Unlock()
	})
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.stopped
}

