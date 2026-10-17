package playback

import (
	"testing"
	"time"
)

func BenchmarkHLSFirstPlaylistWake(b *testing.B) {
	var total time.Duration
	for index := 0; b.Loop(); index++ {
		total += measureHLSPublicationWake(b, index)
	}
	b.ReportMetric(float64(total.Microseconds())/float64(b.N), "wake-µs/op")
}

func BenchmarkHLSSeekSegmentWake(b *testing.B) {
	var total time.Duration
	var probes int64
	for index := 0; b.Loop(); index++ {
		latency, count := measureHLSSegmentWake(b, index)
		total += latency
		probes += count
	}
	b.ReportMetric(float64(total.Microseconds())/float64(b.N), "wake-µs/op")
	b.ReportMetric(float64(probes)/float64(b.N), "probes/op")
}
