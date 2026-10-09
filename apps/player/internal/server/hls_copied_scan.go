package server

import (
	"bufio"
	"context"
	"io"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/MikeO7/kinosail/packages/library"
)

func watchCopiedHLSAdoption(scan context.Context, preparation *startupEncoding, cancel context.CancelFunc) func() {
	done := make(chan struct{})
	go func() {
		ticker := time.NewTicker(25 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-scan.Done():
				return
			case <-ticker.C:
				if preparation.adopted.Load() {
					cancel()
					return
				}
			}
		}
	}()
	return func() { close(done) }
}

func (manager *hlsManager) selectCopiedHLSTimeline(ctx context.Context, item library.Item, recipe hlsRecipe, preparation *startupEncoding, timeline *copiedHLSTimeline) (*copiedHLSTimeline, error) {
	first := -1
	for number := range timeline.Keys {
		if math.Abs(timeline.point(number)-recipe.offset) <= 0.000001 {
			first = number
			break
		}
	}
	if first < 0 {
		return nil, errCopiedHLSIndex
	}
	timeline.Keys = timeline.Keys[first:]
	if preparation.adopted.Load() || manager.validateHLSPolicy(ctx, item, recipe, timeline.Policy) != nil {
		return nil, errCopiedHLSIndex
	}
	manager.mu.Lock()
	idle := len(manager.jobs) == 0 || manager.copiedAACWorkerCurrentLocked(ctx, hlsRecipeKey(item.ID, recipe), timeline.Policy)
	manager.mu.Unlock()
	if !idle {
		return nil, errCopiedHLSIndex
	}
	return timeline, nil
}

func (timeline *copiedHLSTimeline) setTimeBase(value string) error {
	numerator, denominator, found := strings.Cut(value, "/")
	a, aErr := strconv.ParseInt(numerator, 10, 32)
	b, bErr := strconv.ParseInt(denominator, 10, 32)
	if !found || aErr != nil || bErr != nil || a <= 0 || b <= 0 {
		return errCopiedHLSIndex
	}
	timeline.Numerator, timeline.Denominator = a, b
	timeline.TimeBase = float64(a) / float64(b)
	return nil
}

func scanCopiedHLSLines(output io.Reader, maximumBytes int64, maximumLines int, visit func(string) error) error {
	var err error
	scanner := bufio.NewScanner(io.LimitReader(output, maximumBytes+1))
	scanner.Buffer(make([]byte, 1024), 1024)
	var total int64
	count := 0
	for scanner.Scan() {
		total += int64(len(scanner.Bytes()) + 1)
		count++
		if total > maximumBytes || count > maximumLines {
			err = errCopiedHLSIndex
			break
		}
		if err = visit(scanner.Text()); err != nil {
			break
		}
	}
	if scanner.Err() != nil {
		err = errCopiedHLSIndex
	}
	return err
}
