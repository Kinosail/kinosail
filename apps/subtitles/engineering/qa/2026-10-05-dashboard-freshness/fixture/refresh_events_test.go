package main

import (
	"bufio"
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

type r16CapturedStream struct {
	target    *r16Target
	response  *http.Response
	cancel    context.CancelFunc
	unlink    func() bool
	queue     chan r16Frame
	done      chan struct{}
	closeOnce sync.Once
	mu        sync.Mutex
	closeErr  error
	bad       bool
	natural   bool
	cancelled bool
}

func (target *r16Target) openEvents(parent context.Context, cursor string, headerBudget time.Duration) r16EventStream {
	target.track(1)
	defer target.track(-1)
	ctx, cancel := context.WithCancel(parent)
	unlink := target.joinLifecycle(cancel)
	stream := &r16CapturedStream{
		target: target, cancel: cancel, unlink: unlink,
		queue: make(chan r16Frame, 16), done: make(chan struct{}),
	}
	target.mu.Lock()
	target.streams[stream] = true
	target.mu.Unlock()
	if headerBudget <= 0 || headerBudget > 5*time.Second || len(cursor) > 20 {
		stream.reject()
		return stream
	}
	if cursor != "" {
		if _, err := strconv.ParseUint(cursor, 10, 64); err != nil {
			stream.reject()
			return stream
		}
	}
	if !target.beginStream(ctx, stream, cursor, headerBudget) {
		stream.reject()
		return stream
	}
	go stream.capture(ctx)
	return stream
}

func (target *r16Target) beginStream(ctx context.Context, stream *r16CapturedStream, cursor string, budget time.Duration) bool {
	request, err := target.eventRequest(ctx, cursor)
	if err != nil {
		return false
	}
	timer := time.AfterFunc(budget, stream.cancel)
	response, requestErr := target.client.Do(request)
	stopped := timer.Stop()
	if requestErr != nil {
		if response != nil && response.Body.Close() != nil {
			target.fail()
		}
		return false
	}
	stream.response = response
	return stopped && ctx.Err() == nil &&
		response.StatusCode == http.StatusOK &&
		response.Header.Get("Content-Type") == "text/event-stream"
}

func (stream *r16CapturedStream) capture(ctx context.Context) {
	defer close(stream.done)
	defer close(stream.queue)
	defer stream.closeBody()
	defer stream.unlink()
	reader := bufio.NewScanner(stream.response.Body)
	reader.Buffer(make([]byte, 4096), 4096)
	frame := r16Frame{}
	for reader.Scan() {
		line := reader.Text()
		if line == "" {
			if frame.Type != "" && !stream.publish(ctx, frame) {
				return
			}
			frame = r16Frame{}
			continue
		}
		if !r16AddFrameLine(&frame, line) {
			stream.fail()
			return
		}
	}
	stream.mu.Lock()
	stream.natural = reader.Err() == nil && ctx.Err() == nil
	if reader.Err() != nil && ctx.Err() == nil {
		stream.bad = true
	}
	stream.mu.Unlock()
}

func r16AddFrameLine(frame *r16Frame, line string) bool {
	if strings.HasPrefix(line, ":") {
		return true
	}
	key, value, found := strings.Cut(line, ":")
	if !found {
		return false
	}
	value = strings.TrimPrefix(value, " ")
	switch key {
	case "retry":
		return value == "3000"
	case "id":
		return r16FrameID(frame, value)
	case "event":
		return r16FrameType(frame, value)
	case "data":
		return r16FrameData(frame, value)
	default:
		return false
	}
}

func (stream *r16CapturedStream) publish(ctx context.Context, frame r16Frame) bool {
	select {
	case stream.queue <- frame:
		return true
	case <-ctx.Done():
		return false
	default:
		stream.fail()
		return false
	}
}

func (stream *r16CapturedStream) Next(ctx context.Context) (r16Frame, error) {
	select {
	case frame, open := <-stream.queue:
		if open {
			return frame, nil
		}
		return r16Frame{}, errors.New("owned R16 event capture ended")
	case <-ctx.Done():
		return r16Frame{}, errors.New("owned R16 event read deadline")
	}
}

func (stream *r16CapturedStream) Status() int {
	if stream.response == nil {
		return 0
	}
	return stream.response.StatusCode
}

func (stream *r16CapturedStream) Headers() http.Header {
	if stream.response == nil {
		return http.Header{}
	}
	return stream.response.Header.Clone()
}

func r16FrameID(frame *r16Frame, value string) bool {
	if frame.ID != "" || len(value) > 20 {
		return false
	}
	frame.ID = value
	return true
}

func r16FrameType(frame *r16Frame, value string) bool {
	if frame.Type != "" || value == "" || len(value) > 64 {
		return false
	}
	frame.Type = value
	return true
}

func r16FrameData(frame *r16Frame, value string) bool {
	if len(frame.Data)+len(value) > 1024 || len(frame.Data) != 0 {
		return false
	}
	frame.Data = []byte(value)
	return true
}

func (target *r16Target) eventRequest(ctx context.Context, cursor string) (*http.Request, error) {
	endpoint, err := target.endpoint(http.MethodGet, "/api/v1/events")
	if err != nil || target.client == nil {
		return nil, errors.New("owned R16 event request unavailable")
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, errors.New("owned R16 event request unavailable")
	}
	target.headers(request, r16Owner)
	request.Header.Set("Accept", "text/event-stream")
	if cursor != "" {
		request.Header.Set("Last-Event-ID", cursor)
	}
	if !target.admitted(request) {
		return nil, errors.New("owned R16 event request unavailable")
	}
	return request, nil
}
