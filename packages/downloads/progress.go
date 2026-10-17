package downloads

import (
	"context"
	"strconv"
	"strings"
)

type progressReporter func(ratio float64)

type progressKey struct{}

func withProgress(ctx context.Context, report progressReporter) context.Context {
	return context.WithValue(ctx, progressKey{}, report)
}

func progressFrom(ctx context.Context) progressReporter {
	report, _ := ctx.Value(progressKey{}).(progressReporter)
	return report
}

type progressWriter struct {
	total  float64
	report progressReporter
	line   []byte
}

func (writer *progressWriter) Write(data []byte) (int, error) {
	for _, value := range data {
		if value != '\n' {
			if len(writer.line) < 64 {
				writer.line = append(writer.line, value)
			}
			continue
		}
		writer.consume(string(writer.line))
		writer.line = writer.line[:0]
	}
	return len(data), nil
}

func (writer *progressWriter) consume(line string) {
	value, found := strings.CutPrefix(line, "out_time_us=")
	if !found {
		return
	}
	microseconds, err := strconv.ParseInt(value, 10, 64)
	if err != nil || microseconds < 0 || writer.total <= 0 {
		return
	}
	writer.report(min(1, float64(microseconds)/1e6/writer.total))
}

func (manager *Manager) progressReporter(job Job) progressReporter {
	last := -1
	return func(ratio float64) {
		percent := min(99, max(0, int(ratio*100)))
		if percent == last {
			return
		}
		last = percent
		manager.mu.Lock()
		current, exists := manager.jobs[job.ID]
		if !exists || current.State != "preparing" || !current.Created.Equal(job.Created) {
			manager.mu.Unlock()
			return
		}
		current.Progress = percent
		manager.jobs[job.ID] = current
		manager.mu.Unlock()
		manager.emit(current)
	}
}
