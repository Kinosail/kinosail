package transcodepolicy

import (
	"context"
	"sync"
	"testing"
)

func TestCheckCoordinatorProjectsPendingAndStoredResults(t *testing.T) {
	lock := &sync.RWMutex{}
	stored := CheckResult{}
	recorded := CheckResult{}
	coordinator := NewCheckCoordinator(CheckCoordinatorDependencies{
		Lock: lock, Result: &stored, FFmpeg: "",
		Resolve:     func() (Settings, error) { return Settings{Codec: "h264", Accelerator: "none"}, nil },
		Pending:     func() Settings { return Settings{Codec: "hevc", Accelerator: "qsv"} },
		BackendName: func(string) string { return "Software" },
		RecordHardware: func(_ Settings, result CheckResult) {
			recorded = result
		},
	})

	if pending := coordinator.Current(); pending.Status != "not-run" || pending.Codec != "hevc" || pending.Backend != "Software" {
		t.Fatalf("pending check = %#v", pending)
	}
	result := coordinator.Run(context.Background())
	if recorded != result || stored != result || coordinator.Current() != result {
		t.Fatalf("stored check = %#v, recorded = %#v, current = %#v", stored, recorded, coordinator.Current())
	}
}
