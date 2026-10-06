package main

import (
	"context"
)

func (stream *r16CapturedStream) reject() {
	stream.fail()
	stream.cancel()
	stream.unlink()
	stream.closeBody()
	close(stream.queue)
	close(stream.done)
}

func (stream *r16CapturedStream) fail() {
	stream.mu.Lock()
	stream.bad = true
	stream.mu.Unlock()
	stream.target.fail()
}

func (stream *r16CapturedStream) closeBody() {
	stream.closeOnce.Do(func() {
		if stream.response == nil {
			return
		}
		err := stream.response.Body.Close()
		stream.mu.Lock()
		stream.closeErr = err
		stream.mu.Unlock()
		if err != nil {
			stream.target.fail()
		}
	})
}

func (stream *r16CapturedStream) WaitClosed(ctx context.Context) bool {
	select {
	case <-stream.done:
		stream.mu.Lock()
		defer stream.mu.Unlock()
		return stream.natural && !stream.cancelled && !stream.bad && stream.closeErr == nil
	case <-ctx.Done():
		return false
	}
}

func (stream *r16CapturedStream) StopAndJoin(ctx context.Context) bool {
	stream.mu.Lock()
	stream.cancelled = true
	stream.mu.Unlock()
	stream.cancel()
	stream.closeBody()
	select {
	case <-stream.done:
		stream.mu.Lock()
		defer stream.mu.Unlock()
		return !stream.bad && stream.closeErr == nil
	case <-ctx.Done():
		stream.target.fail()
		return false
	}
}

func (target *r16Target) stop(ctx context.Context) bool {
	target.stopOnce.Do(func() {
		target.mu.Lock()
		streams := make([]*r16CapturedStream, 0, len(target.streams))
		for stream := range target.streams {
			streams = append(streams, stream)
		}
		target.mu.Unlock()
		streamsJoined := true
		readersJoined := true
		for _, stream := range streams {
			streamsJoined = stream.StopAndJoin(ctx) && streamsJoined
			readersJoined = stream.joined() && readersJoined
		}
		if target.client != nil {
			target.client.CloseIdleConnections()
		}
		done := make(chan struct{})
		go func() {
			target.server.Close()
			close(done)
		}()
		select {
		case <-done:
		case <-ctx.Done():
			target.fail()
			return
		}
		for {
			target.mu.Lock()
			active := target.active
			target.mu.Unlock()
			if active == 0 {
				break
			}
			select {
			case <-target.changed:
			case <-ctx.Done():
				target.fail()
				return
			}
		}
		closeErr := target.listener.Close()
		target.mu.Lock()
		target.joined = readersJoined && closeErr == nil
		target.stopped = target.joined && streamsJoined && !target.bad
		target.mu.Unlock()
	})
	target.mu.Lock()
	defer target.mu.Unlock()
	return target.stopped
}

func (stream *r16CapturedStream) joined() bool {
	select {
	case <-stream.done:
		return true
	default:
		return false
	}
}

func (target *r16Target) settled() bool {
	target.mu.Lock()
	defer target.mu.Unlock()
	return target.joined
}
