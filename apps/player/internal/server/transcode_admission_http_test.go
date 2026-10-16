package server

import (
	"net/http"
	"os"
	"strings"
	"testing"
)

func phaseCopyBesideTranscode(t *testing.T) {
	fixture := phaseFixture(t, 2, true)
	first := fixture.request(t, 0, "t-a0-s0-none-t0-b0", "copy-first")
	phaseWait(t, func() bool {
		data, _ := os.ReadFile(fixture.starts)
		return strings.Count(string(data), "started") == 1
	})
	second := fixture.request(t, 1, "a-a0-s0-none-t0-b0", "copy-second")
	phaseWait(t, func() bool {
		data, _ := os.ReadFile(fixture.starts)
		return strings.Count(string(data), "started") == 2
	})
	if phaseCount(fixture.events("copy-second"), "HLS transcode started") != 1 {
		t.Fatal("stream copy did not start while the transcode held the CPU")
	}
	if err := os.WriteFile(fixture.release, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if status := phaseStatus(t, second); status != http.StatusOK {
		t.Fatalf("stream copy status = %d", status)
	}
	if status := phaseStatus(t, first); status != http.StatusOK {
		t.Fatalf("transcode status = %d", status)
	}
}
