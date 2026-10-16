package operations

import (
	"fmt"
	"io"
	"math"
	"runtime/metrics"
)

var runtimeGauges = []struct{ metric, name string }{
	{"/gc/heap/live:bytes", "kinosail_go_heap_live_bytes"},
	{"/memory/classes/total:bytes", "kinosail_go_memory_total_bytes"},
	{"/gc/gomemlimit:bytes", "kinosail_go_memory_limit_bytes"},
	{"/sched/goroutines:goroutines", "kinosail_go_goroutines"},
	{"/sched/gomaxprocs:threads", "kinosail_go_gomaxprocs"},
	{"/gc/cycles/total:gc-cycles", "kinosail_go_gc_cycles_total"},
}

var runtimeSummaries = []struct{ metric, name string }{
	{"/sched/pauses/total/gc:seconds", "kinosail_go_gc_pause_seconds"},
	{"/sched/latencies:seconds", "kinosail_go_sched_latency_seconds"},
}

var summaryQuantiles = []struct {
	label string
	value float64
}{{"0.5", 0.5}, {"0.99", 0.99}, {"1", 1}}

func writeRuntimeMetrics(writer io.Writer) {
	samples := make([]metrics.Sample, 0, len(runtimeGauges)+len(runtimeSummaries))
	for _, gauge := range runtimeGauges {
		samples = append(samples, metrics.Sample{Name: gauge.metric})
	}
	for _, summary := range runtimeSummaries {
		samples = append(samples, metrics.Sample{Name: summary.metric})
	}
	metrics.Read(samples)
	for index, gauge := range runtimeGauges {
		if value := samples[index].Value; value.Kind() == metrics.KindUint64 {
			_, _ = fmt.Fprintf(writer, "%s %d\n", gauge.name, value.Uint64())
		}
	}
	for index, summary := range runtimeSummaries {
		if value := samples[len(runtimeGauges)+index].Value; value.Kind() == metrics.KindFloat64Histogram {
			writeSummary(writer, summary.name, value.Float64Histogram())
		}
	}
}

func writeSummary(writer io.Writer, name string, histogram *metrics.Float64Histogram) {
	var total uint64
	for _, count := range histogram.Counts {
		total += count
	}
	for _, quantile := range summaryQuantiles {
		_, _ = fmt.Fprintf(writer, "%s{quantile=%q} %g\n", name, quantile.label, histogramQuantile(histogram, total, quantile.value))
	}
	_, _ = fmt.Fprintf(writer, "%s_count %d\n", name, total)
}

func histogramQuantile(histogram *metrics.Float64Histogram, total uint64, quantile float64) float64 {
	if total == 0 {
		return 0
	}
	target := uint64(math.Ceil(quantile * float64(total)))
	var seen uint64
	for index, count := range histogram.Counts {
		seen += count
		if count == 0 || seen < target {
			continue
		}
		if upper := histogram.Buckets[index+1]; !math.IsInf(upper, 1) {
			return upper
		}
		return math.Max(histogram.Buckets[index], 0)
	}
	return 0
}
