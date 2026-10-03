package server

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/MikeO7/kinosail/packages/identitycore"
	"github.com/MikeO7/kinosail/packages/library"
	"github.com/MikeO7/kinosail/packages/playback"
)

const startupQueueLimit = 3

type startupEncodingKey struct{}

// Shared ownership exists before an encoder so playback can adopt cache refills.
type startupEncoding struct{ adopted atomic.Bool }

type startupRequest struct {
	request  *http.Request
	item     library.Item
	recipe   hlsRecipe
	key      string
	viewer   string
	direct   bool
	encoding *startupEncoding
	expires  time.Time
}

type startupSession struct {
	expires  time.Time
	sequence int64
	playing  bool
}

type startupPreparation struct {
	hls            *hlsManager
	auth           *authentication
	limit          int64
	mu             sync.Mutex
	queue          []startupRequest
	running        bool
	current        *startupRequest
	cancel         context.CancelFunc
	active         map[string]startupSession
	lastMedia      time.Time
	directRequests int
}

func newStartupPreparation(hls *hlsManager, auth *authentication, limit int64) *startupPreparation {
	if limit <= 0 {
		limit = 10_737_418_240 // Same default as the existing cache maintenance policy.
	}
	return &startupPreparation{hls: hls, auth: auth, limit: limit, active: make(map[string]startupSession)}
}

func (startup *startupPreparation) enqueue(value startupRequest) string {
	if !value.direct && startup.hls.startupWindowReady(value.item, value.recipe) {
		return "ready"
	}
	startup.mu.Lock()
	defer startup.mu.Unlock()
	if startup.current != nil && startup.current.key == value.key {
		return "queued"
	}
	for _, queued := range startup.queue {
		if queued.key == value.key {
			return "queued"
		}
	}
	if len(startup.queue) >= startupQueueLimit {
		return "busy"
	}
	value.expires = time.Now().Add(30 * time.Second)
	value.encoding = &startupEncoding{}
	startup.queue = append(startup.queue, value)
	if !startup.running {
		startup.running = true
		go startup.run()
	}
	return "queued"
}

func (startup *startupPreparation) run() {
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()
	for {
		startup.mu.Lock()
		for len(startup.queue) > 0 && time.Now().After(startup.queue[0].expires) {
			startup.queue = startup.queue[1:]
		}
		if len(startup.queue) == 0 || startup.hls.ctx.Err() != nil {
			startup.queue = nil
			startup.running = false
			startup.mu.Unlock()
			return
		}
		startup.mu.Unlock()
		if startup.idle() {
			startup.next()
		}
		select {
		case <-startup.hls.ctx.Done():
		case <-ticker.C:
		}
	}
}

func (startup *startupPreparation) idle() bool {
	startup.mu.Lock()
	idle := startup.idleLocked()
	startup.mu.Unlock()
	startup.hls.mu.Lock()
	idle = idle && len(startup.hls.jobs) == 0
	startup.hls.mu.Unlock()
	if startup.hls.workloads != nil {
		metrics := startup.hls.workloads.Metrics()
		idle = idle && metrics.ActivePlayback == 0 && metrics.WaitingPlayback == 0
	}
	return idle
}

func (startup *startupPreparation) idleLocked() bool {
	now := time.Now()
	playing := false
	for session, state := range startup.active {
		if now.After(state.expires) {
			delete(startup.active, session)
		} else {
			playing = playing || state.playing
		}
	}
	return !playing && startup.directRequests == 0 && now.Sub(startup.lastMedia) >= 2*time.Second
}

func (startup *startupPreparation) next() {
	startup.mu.Lock()
	if len(startup.queue) == 0 || !startup.idleLocked() {
		startup.mu.Unlock()
		return
	}
	value := startup.queue[0]
	startup.queue = startup.queue[1:]
	// Retain trusted cookie/application/connection scope while dropping only the
	// ended HTTP request's cancellation. Server shutdown still stops preparation.
	ctx, cancel := context.WithTimeout(context.WithoutCancel(value.request.Context()), 12*time.Second)
	stop := context.AfterFunc(startup.hls.ctx, cancel)
	startup.current, startup.cancel = &value, cancel
	startup.mu.Unlock()
	defer func() {
		stop()
		cancel()
		startup.mu.Lock()
		startup.current, startup.cancel = nil, nil
		startup.mu.Unlock()
	}()
	request := value.request.Clone(ctx)
	viewer, authenticated := startup.auth.identity(request)
	if !authenticated || viewer.ID != value.viewer || startup.auth.viewerAccess(viewer, request, "POST /api/v1/items/{id}/playback-prepare") != identitycore.Allowed {
		slog.InfoContext(ctx, "HLS startup preparation", "request_id", requestActivityID(ctx), "state", "authorization-expired")
		return
	}
	request = request.WithContext(context.WithValue(ctx, viewerContextKey{}, viewer))
	item, found := visibleItem(request, startup.hls.index, value.item.ID)
	if !found || !viewer.Permits("stream", true) || !value.direct && !hlsAllowed(request) {
		slog.InfoContext(ctx, "HLS startup preparation", "request_id", requestActivityID(ctx), "state", "permission-changed")
		return
	}
	if value.direct {
		startup.hls.prepareDirectRanges(ctx, item)
		return
	}
	if !startup.cacheHeadroom(ctx) {
		slog.InfoContext(ctx, "HLS startup preparation", "request_id", requestActivityID(ctx), "state", "cache-headroom")
		return
	}
	startup.hls.prepareStartupWindow(ctx, item, value.recipe, value.encoding)
}

func (startup *startupPreparation) cancelItem(viewer, id string) {
	startup.mu.Lock()
	defer startup.mu.Unlock()
	kept := startup.queue[:0]
	for _, value := range startup.queue {
		if value.viewer != viewer || value.item.ID != id {
			kept = append(kept, value)
		}
	}
	startup.queue = kept
	if startup.current != nil && startup.current.viewer == viewer && startup.current.item.ID == id {
		startup.cancel()
	}
}

func (startup *startupPreparation) playback(key string) {
	if startup == nil {
		return
	}
	startup.mu.Lock()
	startup.lastMedia = time.Now()
	startup.queue = nil
	if startup.current != nil && startup.current.key == key {
		startup.current.encoding.adopted.Store(true)
	}
	if startup.current != nil && startup.current.key != key {
		startup.cancel()
	}
	startup.mu.Unlock()
	startup.hls.mu.Lock()
	if job := startup.hls.jobs[key]; job != nil && job.preparation != nil {
		job.preparation.adopted.Store(true)
	}
	_ = os.Remove(filepath.Join(startup.hls.cache, key, ".startup")) //nolint:gosec // key is a validated stream cache identity.
	startup.hls.mu.Unlock()
}

func (startup *startupPreparation) beginDirect(key string) func() {
	if startup == nil {
		return func() {}
	}
	startup.mu.Lock()
	startup.directRequests++
	startup.mu.Unlock()
	startup.playback(key)
	return func() {
		startup.mu.Lock()
		startup.directRequests--
		startup.lastMedia = time.Now()
		startup.mu.Unlock()
	}
}

func (startup *startupPreparation) observe(request *http.Request, event playback.TraceEvent) {
	if startup == nil {
		return
	}
	session := currentViewer(request).ID + ":" + event.Session
	startup.mu.Lock()
	defer startup.mu.Unlock()
	state := startup.active[session]
	if event.Sequence <= state.sequence {
		return
	}
	switch event.Event {
	case "pause", "ended", "session-end", "play-request", "playing", "first-moving-frame", "heartbeat":
	default:
		return
	}
	state = startupSession{expires: time.Now().Add(35 * time.Second), sequence: event.Sequence,
		playing: event.Event == "play-request" || !event.Paused && event.Event != "ended" && event.Event != "session-end" && event.Event != "pause"}
	if _, exists := startup.active[session]; exists || len(startup.active) < 64 {
		startup.active[session] = state
	} else if state.playing {
		startup.lastMedia = time.Now().Add(33 * time.Second)
	}
	if state.playing {
		startup.queue = nil
		if startup.cancel != nil {
			startup.cancel()
		}
	}
}
