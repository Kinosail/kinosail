package server

import (
	"context"
	"errors"
	"net/http"
	"sync"
	"time"
)

var errSubtitleBusy = errors.New("another subtitle operation is running; review its outcome before starting another")

type subtitleAdmission struct {
	mu         sync.Mutex
	legacy     int
	background int
	exclusive  bool
	yieldUntil time.Time
}

const subtitleBackgroundYield = 2 * time.Minute

type subtitleAdmissionKey struct{}

func (gate *subtitleAdmission) enter(ctx context.Context) (context.Context, func(), error) {
	if gate == nil || ctx.Value(subtitleAdmissionKey{}) == gate {
		return ctx, func() {}, nil
	}
	gate.mu.Lock()
	defer gate.mu.Unlock()
	if gate.exclusive {
		return ctx, nil, errSubtitleBusy
	}
	gate.legacy++
	return context.WithValue(ctx, subtitleAdmissionKey{}, gate), func() {
		gate.mu.Lock()
		defer gate.mu.Unlock()
		gate.legacy--
	}, nil
}

func (gate *subtitleAdmission) enterBackground(ctx context.Context) (context.Context, func(), error) {
	if gate == nil || ctx.Value(subtitleAdmissionKey{}) == gate {
		return ctx, func() {}, nil
	}
	gate.mu.Lock()
	defer gate.mu.Unlock()
	if gate.exclusive || time.Now().Before(gate.yieldUntil) {
		return ctx, nil, errSubtitleBusy
	}
	gate.legacy++
	gate.background++
	return context.WithValue(ctx, subtitleAdmissionKey{}, gate), func() {
		gate.mu.Lock()
		defer gate.mu.Unlock()
		gate.legacy--
		gate.background--
	}, nil
}

func (gate *subtitleAdmission) claim() bool {
	gate.mu.Lock()
	defer gate.mu.Unlock()
	if gate.exclusive || gate.legacy != 0 {
		if !gate.exclusive && gate.legacy == gate.background {
			gate.yieldUntil = time.Now().Add(subtitleBackgroundYield)
		}
		return false
	}
	gate.exclusive, gate.yieldUntil = true, time.Time{}
	return true
}

func (gate *subtitleAdmission) release() {
	gate.mu.Lock()
	defer gate.mu.Unlock()
	gate.exclusive = false
}

func (gate *subtitleAdmission) legacyHandler(next http.HandlerFunc) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		ctx, settled, err := gate.enter(request.Context())
		if err != nil {
			apiError(writer, err, http.StatusConflict)
			return
		}
		defer settled()
		next(writer, request.WithContext(ctx))
	}
}
