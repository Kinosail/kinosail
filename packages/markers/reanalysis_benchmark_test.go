package markers

import "testing"

func BenchmarkMarkerReanalysisAfterOneAddition(b *testing.B) {
	items := libraryMovies(1001)
	for b.Loop() {
		b.StopTimer()
		analyzer, counter := countingAnalyzer(b, "")
		analyzer.cache = b.TempDir()
		analyzer.file = ""
		if err := analyzer.analyzeBatch(b.Context(), items[:1000], nil); err != nil {
			b.Fatal(err)
		}
		before := counter.total()
		b.StartTimer()
		changed := analyzer.changedItems(items)
		analyzer.enqueueChanged(items)
		job := <-analyzer.jobs
		if err := analyzer.analyzeBatch(b.Context(), job.items, job.targets); err != nil {
			b.Fatal(err)
		}
		b.ReportMetric(float64(len(changed)), "changed-titles/op")
		b.ReportMetric(float64(counter.total()-before), "extractions/op")
	}
}
