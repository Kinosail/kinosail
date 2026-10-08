package server_test

import (
	"context"
	"io"
	"log"
	"log/slog"
	"net/http"
	"sync"
	"testing"
)

const remainingPublicRequestID = "00000517-0000-4000-8000-000000000001"

type remainingPublicPublication struct {
	mu        sync.Mutex
	completed int
}

type remainingPublicCompletionHandler struct {
	slog.Handler
	publication *remainingPublicPublication
}

func remainingPublicObservePublication(t *testing.T) *remainingPublicPublication {
	t.Helper()
	previous := slog.Default()
	previousOutput, previousFlags := log.Writer(), log.Flags()
	publication := &remainingPublicPublication{}
	// A standalone sink avoids reinstalling slog's default log bridge recursively.
	handler := slog.NewJSONHandler(io.Discard, nil)
	slog.SetDefault(slog.New(remainingPublicCompletionHandler{Handler: handler, publication: publication}))
	t.Cleanup(func() {
		slog.SetDefault(previous)
		log.SetOutput(previousOutput)
		log.SetFlags(previousFlags)
	})
	return publication
}

func (handler remainingPublicCompletionHandler) Handle(ctx context.Context, record slog.Record) error {
	if record.Message == "HLS transcode completed" {
		requestID := ""
		record.Attrs(func(attribute slog.Attr) bool {
			if attribute.Key == "request_id" {
				requestID = attribute.Value.String()
			}
			return true
		})
		if requestID == remainingPublicRequestID {
			handler.publication.mu.Lock()
			handler.publication.completed++
			handler.publication.mu.Unlock()
		}
	}
	return handler.Handler.Handle(ctx, record)
}

func (handler remainingPublicCompletionHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return remainingPublicCompletionHandler{Handler: handler.Handler.WithAttrs(attrs), publication: handler.publication}
}

func (handler remainingPublicCompletionHandler) WithGroup(name string) slog.Handler {
	return remainingPublicCompletionHandler{Handler: handler.Handler.WithGroup(name), publication: handler.publication}
}

func (publication *remainingPublicPublication) hasCompleted(expected int) bool {
	publication.mu.Lock()
	defer publication.mu.Unlock()
	return publication.completed == expected
}

// Each fixture response is bounded before buffering any incoming write.
type remainingPublicResponse struct {
	header http.Header
	code   int
	body   remainingPublicBoundedOutput
}

func (response *remainingPublicResponse) Header() http.Header { return response.header }

func (response *remainingPublicResponse) WriteHeader(code int) {
	if response.code == 0 {
		response.code = code
	}
}

func (response *remainingPublicResponse) Write(data []byte) (int, error) {
	if response.code == 0 {
		response.code = http.StatusOK
	}
	return response.body.Write(data)
}
