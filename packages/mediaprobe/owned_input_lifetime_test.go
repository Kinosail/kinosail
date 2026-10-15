package mediaprobe

import (
	"context"
	"errors"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestInputProbeJoinsBlockedInputAndDescendant(t *testing.T) {
	if runtime.GOOS != "linux" && runtime.GOOS != "darwin" {
		ctx, cancel := context.WithTimeout(t.Context(), time.Second)
		defer cancel()
		err := ProbeInputLines(ctx, nil, "must-not-start", nil, strings.NewReader("input"), 1024, 1, func(string) error { t.Fatal("unsupported input launched a probe"); return nil })
		if !errors.Is(err, ErrProbeInputUnsupported) {
			t.Fatal("unsupported process input did not reject before launch")
		}
		return
	}
	executable := copiedHLSProbeExecutable(t)
	for _, cancelNow := range []bool{false, true} {
		name := "deadline"
		if cancelNow {
			name = "explicit cancellation"
		}
		t.Run(name, func(t *testing.T) {
			err := verifyCopiedHLSProbeLifetime(t, executable, "held", false, cancelNow, true, strings.NewReader(strings.Repeat("x", 1<<20)))
			if cancelNow && !errors.Is(err, context.Canceled) {
				t.Fatal("input probe lost cancellation after settlement")
			}
		})
	}
}
