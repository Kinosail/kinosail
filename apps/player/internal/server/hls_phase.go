package server

import (
	"context"
	"errors"
	"log/slog"
	"math"
	"sync"
	"time"

	"github.com/MikeO7/kinosail/packages/workload"
)

type hlsObservationKey struct{}

type hlsObservation struct {
	mu                            sync.Mutex
	requestID, mode, phase, class string
	started, queueStarted         time.Time
	queueMS, seekMS               int64
	cost, capacity, segment       int
	ready, launched, pendingReady bool
}

func newHLSObservation(requestID string, segment int) *hlsObservation {
	return &hlsObservation{requestID: requestID, segment: segment, phase: "queued", class: "unknown", seekMS: -1, started: time.Now()}
}

func hlsObservationFor(ctx context.Context) *hlsObservation {
	observation, _ := ctx.Value(hlsObservationKey{}).(*hlsObservation)
	return observation
}

func (observation *hlsObservation) queued(mode string) {
	if observation == nil {
		return
	}
	observation.mu.Lock()
	switch mode {
	case "remux", "audio-transcode", "transcode":
		observation.mode = mode
	default:
		observation.mode = "unknown"
	}
	observation.mu.Unlock()
	observation.emit("queued", "")
}

func (manager *hlsManager) acquireHLSEncode(ctx context.Context, cost int, device string, seek float64, segment int) (func(), error) {
	observation := hlsObservationFor(ctx)
	class := startupWorkClass(ctx)
	if observation != nil {
		observation.mu.Lock()
		observation.cost, observation.capacity, observation.segment = cost, manager.workloads.EncodingCapacity(), segment
		observation.class = "playback"
		if class == workload.Background {
			observation.class = "background"
		}
		observation.seekMS = -1
		if !math.IsNaN(seek) && !math.IsInf(seek, 0) && seek >= 0 && seek <= 7*24*60*60 {
			observation.seekMS = int64(math.Round(seek * 1000))
		}
		observation.queueStarted = time.Now()
		observation.mu.Unlock()
		observation.emit("admission_wait", "")
	}
	release, err := manager.workloads.AcquireEncoding(ctx, class, cost, device)
	if observation != nil {
		observation.mu.Lock()
		observation.queueMS = time.Since(observation.queueStarted).Milliseconds()
		observation.mu.Unlock()
		if err != nil {
			observation.emit("admission_rejected", hlsPhaseOutcome(err))
		} else {
			observation.emit("admitted", "")
		}
	}
	return release, err
}

func (observation *hlsObservation) emit(phase, outcome string) {
	if observation == nil {
		return
	}
	observation.mu.Lock()
	if phase == "media_ready" {
		if !observation.launched {
			observation.pendingReady = true
			observation.mu.Unlock()
			return
		}
		if observation.ready {
			observation.mu.Unlock()
			return
		}
		observation.ready = true
		observation.pendingReady = false
	}
	if phase == "process_started" {
		observation.launched = true
	}
	observation.phase = phase
	fields := []any{"request_id", observation.requestID, "phase", phase, "mode", observation.mode, "work_class", observation.class, "encoder_cost", observation.cost, "capacity", observation.capacity, "input_seek_ms", observation.seekMS, "segment_start", observation.segment, "elapsed_ms", time.Since(observation.started).Milliseconds(), "queue_wait_ms", observation.queueMS}
	if outcome != "" {
		fields = append(fields, "outcome", outcome)
	}
	message := "HLS encode phase"
	if phase == "process_started" {
		message = "HLS transcode started"
	}
	slog.Info(message, fields...)
	flushReady := phase == "process_started" && observation.pendingReady
	observation.mu.Unlock()
	if flushReady {
		observation.emit("media_ready", "")
	}
}

func hlsPhaseOutcome(err error) string {
	if errors.Is(err, context.Canceled) {
		return "canceled"
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "deadline"
	}
	return "failed"
}

type hlsReadinessError struct {
	cause error
	phase string
}

func (failure *hlsReadinessError) Error() string { return failure.cause.Error() }
func (failure *hlsReadinessError) Unwrap() error { return failure.cause }

func observedHLSReadinessError(err error, job *hlsJob) error {
	if err == nil || job == nil || job.observation == nil {
		return err
	}
	job.observation.mu.Lock()
	phase := job.observation.phase
	job.observation.mu.Unlock()
	return &hlsReadinessError{cause: err, phase: phase}
}

func hlsReadinessPhase(err error) string {
	var failure *hlsReadinessError
	if errors.As(err, &failure) {
		return failure.phase
	}
	return "not_queued"
}
