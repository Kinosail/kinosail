package operations

import (
	"net/http"
	"sync/atomic"
	"time"
)

type RequestTelemetry struct {
	requests atomic.Uint64
	failed   atomic.Uint64
	panics   atomic.Uint64
	failures FailureLog
}

func (telemetry *RequestTelemetry) Observe(requestID, method, path string, status int, duration time.Duration, panicked bool) {
	telemetry.failures.Record(requestID, method, path, status, duration)
	telemetry.requests.Add(1)
	if status >= http.StatusBadRequest {
		telemetry.failed.Add(1)
	}
	if panicked {
		telemetry.panics.Add(1)
	}
}

func (telemetry *RequestTelemetry) Counts() (uint64, uint64, uint64) {
	return telemetry.requests.Load(), telemetry.failed.Load(), telemetry.panics.Load()
}

func (telemetry *RequestTelemetry) Recent() []FailureEvent { return telemetry.failures.Recent() }
