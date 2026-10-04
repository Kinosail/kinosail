package server

import (
	"context"
	"errors"
	"net/http"
	"sync"
)

var errSubtitleBusy = errors.New("another subtitle operation is running; review its outcome before starting another")

// Legacy work remains concurrent. Receipt work is exclusive with every legacy
// writer and audio consumer, including work waiting for the audio seam.
type subtitleAdmission struct {
	mu        sync.Mutex
	legacy    int
	exclusive bool
}

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

func (gate *subtitleAdmission) claim() bool {
	gate.mu.Lock()
	defer gate.mu.Unlock()
	if gate.exclusive || gate.legacy != 0 {
		return false
	}
	gate.exclusive = true
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
