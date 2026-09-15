package dashboard

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync/atomic"
	"testing"
	"time"
)

func TestCanceledQueuedFlightNeverStartsAndAllowsAFreshProbe(t *testing.T) {
	started, release := make(chan struct{}, probeConcurrency), make(chan struct{})
	var fifthRequests atomic.Int64
	apps := make([]App, 0, probeConcurrency+1)
	for index := 1; index <= probeConcurrency+1; index++ {
		apps = append(apps, testApp(index, fmt.Sprintf("http://probe.example.test/%d", index), true))
	}
	service, _ := serviceForTest(t, testBoard(apps...))
	prober := NewProber(service, 30*time.Second, nil)
	prober.client.Transport = probeRoundTrip(func(request *http.Request) (*http.Response, error) {
		if request.URL.Path == "/5" {
			fifthRequests.Add(1)
			return probeResponse(request), nil
		}
		started <- struct{}{}
		<-release
		return probeResponse(request), nil
	})
	results := make(chan probeResult, probeConcurrency)
	for index := range probeConcurrency {
		go sendProbeResult(prober, context.Background(), apps[index].ID, results)
	}
	for range probeConcurrency {
		<-started
	}
	ctx, cancel := context.WithCancel(context.Background())
	fifth := make(chan probeResult, 1)
	go sendProbeResult(prober, ctx, apps[probeConcurrency].ID, fifth)
	waitForProbeWaiters(t, prober, apps[probeConcurrency].ID, 1)
	cancel()
	if got := <-fifth; !errors.Is(got.err, context.Canceled) {
		t.Fatalf("canceled queued caller result = %+v", got)
	}
	close(release)
	waitForProbeFlightGone(t, prober, apps[probeConcurrency].ID)
	if got := fifthRequests.Load(); got != 0 {
		t.Fatalf("canceled queued flight started %d requests", got)
	}
	assertFreshQueuedProbe(t, prober, apps[probeConcurrency].ID, &fifthRequests)
	for range probeConcurrency {
		<-results
	}
}

func assertFreshQueuedProbe(t *testing.T, prober *Prober, id string, fifthRequests *atomic.Int64) {
	t.Helper()
	if health, err := prober.ProbeOne(t.Context(), id); err != nil || health.State != "reachable" {
		t.Fatalf("fresh probe result = %+v, %v", health, err)
	}
	if got := fifthRequests.Load(); got != 1 {
		t.Fatalf("fresh caller started %d requests", got)
	}
}

func waitForProbeFlightGone(t *testing.T, prober *Prober, id string) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		prober.flightMu.Lock()
		found := false
		for key := range prober.flights {
			found = found || key.id == id
		}
		prober.flightMu.Unlock()
		if !found {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("canceled queued flight was not removed")
}
