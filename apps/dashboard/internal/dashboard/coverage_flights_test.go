package dashboard

import (
	"context"
	"errors"
	"net"
	"net/http"
	"sync"
	"testing"
	"time"
)

type coverageDispatchedContext struct {
	context.Context
	checked chan struct{}
	release chan struct{}
	once    *sync.Once
}

func (ctx coverageDispatchedContext) Err() error {
	ctx.once.Do(func() {
		close(ctx.checked)
		<-ctx.release
	})
	return ctx.Context.Err()
}

func TestCoverageCancellationAfterDispatchSkipsJob(t *testing.T) {
	service, _ := serviceForTest(t, testBoard(testApp(1, "http://127.0.0.1:1", true)))
	prober := NewProber(service, time.Hour, nil)
	parent, cancel := context.WithCancel(t.Context())
	t.Cleanup(cancel)
	ctx := coverageDispatchedContext{Context: parent, checked: make(chan struct{}), release: make(chan struct{}), once: &sync.Once{}}
	result := make(chan ProbeBatch, 1)
	go func() { result <- prober.ProbeAll(ctx) }()
	<-ctx.checked
	cancel()
	close(ctx.release)
	batch := <-result
	if !batch.Canceled || batch.Completed != 0 || service.Snapshot().Apps[0].Health.State != "unchecked" {
		t.Fatalf("canceled dispatched job = %+v", batch)
	}
}

func TestCoverageStaleFlightGuards(t *testing.T) {
	service, _ := serviceForTest(t, testBoard())
	prober := NewProber(service, time.Hour, nil)
	key := probeFlightKey{id: "missing"}
	flight := &probeFlight{waiters: 1}
	prober.leaveFlight(key, flight)
	if flight.waiters != 1 || prober.startFlight(key, flight) {
		t.Fatal("stale flight changed state")
	}
	if _, _, err := service.beginProbe(key); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing probe = %v", err)
	}
	key.revision = 1
	if _, _, err := service.beginProbe(key); !errors.Is(err, ErrConflict) {
		t.Fatalf("stale probe = %v", err)
	}
}

func TestCoverageFlightWaitAndSnapshotFailures(t *testing.T) {
	for _, mode := range []string{"slot", "missing", "stale"} {
		t.Run(mode, func(t *testing.T) {
			service, _ := serviceForTest(t, testBoard())
			prober := NewProber(service, time.Hour, nil)
			ctx, cancel := context.WithCancel(t.Context())
			t.Cleanup(cancel)
			key := probeFlightKey{id: "missing"}
			flight := &probeFlight{done: make(chan struct{}), cancel: cancel, waiters: 1}
			prober.flights[key] = flight
			switch mode {
			case "slot":
				prober.slots = make(chan struct{})
				cancel()
			case "stale":
				delete(prober.flights, key)
			}
			prober.runFlight(ctx, key, flight)
			if mode == "stale" {
				return
			}
			if flight.err == nil || len(prober.flights) != 0 {
				t.Fatalf("failed flight = %v", flight.err)
			}
			assertCoverageFlightFinished(t, flight)
		})
	}
}

func assertCoverageFlightFinished(t *testing.T, flight *probeFlight) {
	t.Helper()
	select {
	case <-flight.done:
	default:
		t.Fatal("failed flight did not finish")
	}
}

func TestCoverageCanceledFlightDoesNotPublishHealth(t *testing.T) {
	app := testApp(1, "http://127.0.0.1:1", true)
	service, _ := serviceForTest(t, testBoard(app))
	prober := NewProber(service, time.Hour, nil)
	ctx, cancel := context.WithCancel(t.Context())
	t.Cleanup(cancel)
	prober.client.Transport = probeRoundTrip(func(*http.Request) (*http.Response, error) { cancel(); return nil, context.Canceled })
	key, _ := service.probeIntent(app.ID)
	flight := &probeFlight{done: make(chan struct{}), cancel: cancel, waiters: 1}
	prober.flights[key] = flight
	prober.runFlight(ctx, key, flight)
	if !errors.Is(flight.err, context.Canceled) || flight.health.State != "" {
		t.Fatalf("canceled flight = %+v", flight)
	}
	if got := explainProbeError(&net.OpError{Op: "read", Err: &net.DNSError{IsTimeout: true}}); got != "Check timed out" {
		t.Fatalf("network timeout = %q", got)
	}
}
