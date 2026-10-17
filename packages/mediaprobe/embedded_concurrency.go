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

var errEmbeddedRetry = errors.New("embedded subtitle extraction must be retried")

type embeddedCall struct {
	done chan struct{}
	data []byte
	err  error
}

type embeddedTarget struct {
	stream int
	path   string
	key    string
	call   *embeddedCall
}

func embeddedCallKey(item library.Item, version string, stream int, path string) string {
	return item.Path + "\x00" + version + "\x00" + strconv.Itoa(stream) + "\x00" + path
}

func (probe *Probe) embeddedData(ctx context.Context, item library.Item, stream int, options EmbeddedOptions, path string, companions []int) ([]byte, error) {
	version := playback.SourceVersion(item.Path)
	key := embeddedCallKey(item, version, stream, path)
	for ctx.Err() == nil {
		probe.mu.Lock()
		call := probe.embeddedCalls[key]
		if call == nil {
			targets := probe.claimEmbedded(item, version, options.CacheDir, embeddedTarget{stream: stream, path: path, key: key}, companions)
			probe.mu.Unlock()
			extractAndCacheEmbedded(ctx, item, targets, options, version)
			probe.settleEmbedded(targets)
			return targets[0].call.data, targets[0].call.err
		}
		probe.mu.Unlock()
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-call.done:
			if !errors.Is(call.err, context.Canceled) && !errors.Is(call.err, context.DeadlineExceeded) && !errors.Is(call.err, errEmbeddedRetry) {
				return call.data, call.err
			}
		}
	}
	return nil, ctx.Err()
}

func (probe *Probe) claimEmbedded(item library.Item, version, cacheDir string, leader embeddedTarget, companions []int) []embeddedTarget {
	if probe.embeddedCalls == nil {
		probe.embeddedCalls = make(map[string]*embeddedCall)
	}
	targets := make([]embeddedTarget, 0, 1+len(companions))
	for position := -1; position < len(companions); position++ {
		target := leader
		if position >= 0 {
			target.stream, target.path = companions[position], embeddedSubtitlePath(cacheDir, item, companions[position])
			target.key = embeddedCallKey(item, version, target.stream, target.path)
			if probe.embeddedCalls[target.key] != nil {
				continue
			}
		}
		target.call = &embeddedCall{done: make(chan struct{})}
		probe.embeddedCalls[target.key] = target.call
		targets = append(targets, target)
	}
	return targets
}

func (probe *Probe) settleEmbedded(targets []embeddedTarget) {
	probe.mu.Lock()
	defer probe.mu.Unlock()
	for _, target := range targets {
		delete(probe.embeddedCalls, target.key)
		close(target.call.done)
	}
}

func extractAndCacheEmbedded(ctx context.Context, item library.Item, targets []embeddedTarget, options EmbeddedOptions, version string) {
	pending := make([]embeddedTarget, 0, len(targets))
	for _, target := range targets {
		if data, err := readEmbeddedSubtitle(target.path); err == nil {
			target.call.data = data
		} else {
			pending = append(pending, target)
		}
	}
	if len(pending) == 0 {
		return
	}
	started := time.Now()
	extractEmbeddedTargets(ctx, item, pending, options)
	for _, target := range pending {
		if target.call.err == nil {
			target.call.data, target.call.err = cacheEmbedded(ctx, item, target, options, version, started)
		} else if !errors.Is(target.call.err, errEmbeddedRetry) {
			target.call.data, target.call.err = nil, fmt.Errorf("extract embedded subtitle: %w", target.call.err)
		}
	}
}

func cacheEmbedded(ctx context.Context, item library.Item, target embeddedTarget, options EmbeddedOptions, version string, started time.Time) ([]byte, error) {
	if playback.SourceVersion(item.Path) != version || target.path != "" && target.path != embeddedSubtitlePath(options.CacheDir, item, target.stream) {
		return nil, errors.New("subtitle source changed during extraction")
	}
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	if target.path != "" {
		if err := privatefile.WriteCache(target.path, target.call.data); err != nil {
			return nil, err
		}
		slog.InfoContext(ctx, "embedded subtitle cached", "stream", target.stream, "bytes", len(target.call.data), "duration_ms", time.Since(started).Milliseconds())
	}
	return target.call.data, nil
}
