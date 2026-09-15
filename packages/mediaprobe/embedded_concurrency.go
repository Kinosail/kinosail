package mediaprobe

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"github.com/MikeO7/kinosail/packages/library"
	"github.com/MikeO7/kinosail/packages/playback"
	"github.com/MikeO7/kinosail/packages/privatefile"
)

type embeddedCall struct {
	done chan struct{}
	data []byte
	err  error
}

// Background preparation and interactive requests share extraction for one source version.
func (probe *Probe) embeddedData(ctx context.Context, item library.Item, stream int, options EmbeddedOptions, path string) ([]byte, error) {
	version := playback.SourceVersion(item.Path)
	key := item.Path + "\x00" + version + "\x00" + strconv.Itoa(stream) + "\x00" + path
	for ctx.Err() == nil {
		probe.mu.Lock()
		call := probe.embeddedCalls[key]
		if call == nil {
			call = &embeddedCall{done: make(chan struct{})}
			if probe.embeddedCalls == nil {
				probe.embeddedCalls = make(map[string]*embeddedCall)
			}
			probe.embeddedCalls[key] = call
			probe.mu.Unlock()
			call.data, call.err = extractAndCacheEmbedded(ctx, item, stream, options, path, version)
			probe.mu.Lock()
			delete(probe.embeddedCalls, key)
			close(call.done)
			probe.mu.Unlock()
			return call.data, call.err
		}
		probe.mu.Unlock()
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-call.done:
			if !errors.Is(call.err, context.Canceled) && !errors.Is(call.err, context.DeadlineExceeded) {
				return call.data, call.err
			}
			// An interrupted background owner must not fail a still-active viewer request.
		}
	}
	return nil, ctx.Err()
}

func extractAndCacheEmbedded(ctx context.Context, item library.Item, stream int, options EmbeddedOptions, path, version string) ([]byte, error) {
	if data, err := readEmbeddedSubtitle(path); err == nil {
		return data, nil
	}
	started := time.Now()
	data, err := extractEmbeddedSubtitle(ctx, options.FFmpeg, item.Path, stream)
	if err != nil {
		return nil, fmt.Errorf("extract embedded subtitle: %w", err)
	}
	if playback.SourceVersion(item.Path) != version || path != "" && path != embeddedSubtitlePath(options.CacheDir, item, stream) {
		return nil, errors.New("subtitle source changed during extraction")
	}
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	if path != "" {
		if err := privatefile.WriteCache(path, data); err != nil {
			return nil, err
		}
		slog.InfoContext(ctx, "embedded subtitle cached", "stream", stream, "bytes", len(data), "duration_ms", time.Since(started).Milliseconds())
	}
	return data, nil
}
