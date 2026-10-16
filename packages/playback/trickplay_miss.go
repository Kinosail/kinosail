package playback

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"time"
)

var errTrickplayMiss = errors.New("trickplay frame is unavailable")

func nonEmptyFrame(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if info.Size() == 0 {
		return errEmptyTrickplayFrame
	}
	return nil
}

func encoderMiss(ctx context.Context, err error) bool {
	var exit *exec.ExitError
	return ctx.Err() == nil && (errors.Is(err, errEmptyTrickplayFrame) || errors.As(err, &exit))
}

func encoderMissClass(err error) string {
	if errors.Is(err, errEmptyTrickplayFrame) {
		return "empty-output"
	}
	return "encoder-exit"
}

func (frames *Trickplay) missed(target string) bool {
	frames.mu.Lock()
	defer frames.mu.Unlock()
	expiry, found := frames.misses[target]
	return found && time.Now().Before(expiry)
}

func (frames *Trickplay) rememberMiss(target string) {
	now := time.Now()
	frames.mu.Lock()
	defer frames.mu.Unlock()
	if frames.misses == nil {
		frames.misses = make(map[string]time.Time)
	}
	for key, expiry := range frames.misses {
		if !now.Before(expiry) {
			delete(frames.misses, key)
		}
	}
	frames.misses[target] = now.Add(trickplayMissTime)
}
