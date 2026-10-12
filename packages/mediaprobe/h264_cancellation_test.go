package mediaprobe

import (
	"context"
	"errors"
	"testing"
)

// A populated journey cannot deterministically cancel between all three scans.
// Each phase must preserve cancellation so optional callers cannot start fallback.
func TestH264CancellationSurvivesEveryPhase(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	probe := &Probe{}
	options := H264Scan{Path: "unused", Duration: 4, MaximumSpan: 4, MaximumPacketBytes: 1024, MaximumPacketLines: 10}
	timeline := &h264Timeline{Keys: []H264Key{{}}, TimeBase: 0.001}
	cases := map[string]func() error{
		"index":         func() error { _, err := probe.H264Index(ctx, "unused", options); return err },
		"packets":       func() error { _, err := probe.scanH264Packets(ctx, options); return err },
		"suffix":        func() error { _, err := probe.scanH264Suffix(ctx, options); return err },
		"idr":           func() error { return certifyH264(ctx, nil, "unused", "unused", timeline) },
		"configuration": func() error { return certifyH264Configuration(ctx, nil, "unused", "unused", timeline) },
	}
	for name, run := range cases {
		t.Run(name, func(t *testing.T) {
			if !errors.Is(run(), context.Canceled) {
				t.Fatal("index phase lost cancellation")
			}
		})
	}
}
