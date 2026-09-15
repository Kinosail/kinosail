package dashboard

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestSharedProbeSurvivesEitherCallerCancellation(t *testing.T) {
	for _, canceledCaller := range []string{"leader", "follower"} {
		t.Run(canceledCaller, func(t *testing.T) {
			entered, release := make(chan struct{}), make(chan struct{})
			server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
				close(entered)
				<-release
				response.WriteHeader(http.StatusNoContent)
			}))
			defer server.Close()
			app := testApp(1, server.URL, true)
			service, _ := serviceForTest(t, testBoard(app))
			prober := NewProber(service, 30*time.Second, nil)
			leaderCtx, cancelLeader := context.WithCancel(context.Background())
			followerCtx, cancelFollower := context.WithCancel(context.Background())
			defer cancelLeader()
			defer cancelFollower()
			leader, follower := make(chan probeResult, 1), make(chan probeResult, 1)
			go sendProbeResult(prober, leaderCtx, app.ID, leader)
			<-entered
			go sendProbeResult(prober, followerCtx, app.ID, follower)
			waitForProbeWaiters(t, prober, app.ID, 2)
			canceled, survivor := leader, follower
			if canceledCaller == "leader" {
				cancelLeader()
			} else {
				cancelFollower()
				canceled, survivor = follower, leader
			}
			if result := <-canceled; !errors.Is(result.err, context.Canceled) {
				t.Fatalf("canceled caller result = %+v", result)
			}
			close(release)
			if result := <-survivor; result.err != nil || result.health.State != "reachable" {
				t.Fatalf("surviving caller result = %+v", result)
			}
		})
	}
}

func TestCanceledOnlyCallerDoesNotCancelItsBoundedProbe(t *testing.T) {
	entered, release := make(chan struct{}), make(chan struct{})
	var requests atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		if requests.Add(1) == 1 {
			close(entered)
		}
		<-release
		response.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()
	app := testApp(1, server.URL, true)
	service, _ := serviceForTest(t, testBoard(app))
	prober := NewProber(service, 30*time.Second, nil)
	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan probeResult, 1)
	go sendProbeResult(prober, ctx, app.ID, result)
	<-entered
	cancel()
	if got := <-result; !errors.Is(got.err, context.Canceled) {
		t.Fatalf("canceled caller result = %+v", got)
	}
	if health := service.Snapshot().Apps[0].Health; health.State == "unavailable" {
		t.Fatalf("caller cancellation published false health: %+v", health)
	}
	survivor := make(chan probeResult, 1)
	go sendProbeResult(prober, context.Background(), app.ID, survivor)
	waitForProbeWaiters(t, prober, app.ID, 1)
	close(release)
	if got := <-survivor; got.err != nil || got.health.State != "reachable" {
		t.Fatalf("later caller result = %+v", got)
	}
	if got := requests.Load(); got != 1 {
		t.Fatalf("callers started %d network requests", got)
	}
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if service.Snapshot().Apps[0].Health.State == "reachable" {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("bounded probe did not finish after its caller left")
}

func TestPreCanceledCallerDoesNotStartAProbe(t *testing.T) {
	var requests atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		response.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()
	app := testApp(1, server.URL, true)
	service, _ := serviceForTest(t, testBoard(app))
	prober := NewProber(service, 30*time.Second, nil)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := prober.ProbeOne(ctx, app.ID); !errors.Is(err, context.Canceled) {
		t.Fatalf("pre-canceled caller error = %v", err)
	}
	time.Sleep(20 * time.Millisecond)
	if got := requests.Load(); got != 0 {
		t.Fatalf("pre-canceled caller started %d requests", got)
	}
}

func TestCanceledBatchDoesNotLaunchQueuedProbes(t *testing.T) {
	started, release := make(chan struct{}, 4), make(chan struct{})
	var blocked atomic.Int64
	apps := []App{
		testApp(1, "http://probe.example.test/fast", true),
		testApp(2, "http://probe.example.test/blocked-2", true),
		testApp(3, "http://probe.example.test/blocked-3", true),
		testApp(4, "http://probe.example.test/blocked-4", true),
		testApp(5, "http://probe.example.test/blocked-5", true),
	}
	service, _ := serviceForTest(t, testBoard(apps...))
	prober := NewProber(service, 30*time.Second, nil)
	prober.client.Transport = probeRoundTrip(func(request *http.Request) (*http.Response, error) {
		if request.URL.Path == "/fast" {
			return probeResponse(request), nil
		}
		blocked.Add(1)
		started <- struct{}{}
		<-release
		return probeResponse(request), nil
	})
	defer close(release)
	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan ProbeBatch, 1)
	go func() { result <- prober.ProbeAll(ctx) }()
	for range 3 {
		<-started
	}
	cancel()
	batch := <-result
	if batch.Completed != 1 || !batch.Canceled {
		t.Fatalf("canceled batch = %+v", batch)
	}
	if got := blocked.Load(); got != 3 {
		t.Fatalf("canceled batch launched %d blocked probes", got)
	}
}

type probeRoundTrip func(*http.Request) (*http.Response, error)

func (roundTrip probeRoundTrip) RoundTrip(request *http.Request) (*http.Response, error) {
	return roundTrip(request)
}

func probeResponse(request *http.Request) *http.Response {
	return &http.Response{StatusCode: http.StatusNoContent, Body: http.NoBody, Header: make(http.Header), Request: request}
}

func TestConfigurationMutationInvalidatesAnAtomicProbeSnapshot(t *testing.T) {
	app := testApp(1, "http://old.example.test", true)
	service, _ := serviceForTest(t, testBoard(app))
	key, found := service.probeIntent(app.ID)
	started, generation, err := service.beginProbe(key)
	if !found || err != nil || started.HealthURL != app.HealthURL {
		t.Fatalf("probe snapshot = %#v, %d, %v, %v", started, generation, found, err)
	}
	newHealthURL := "http://new.example.test"
	if _, _, err := service.Update(context.Background(), app.ID, UpdateInput{HealthURL: &newHealthURL, ExpectedVersion: 1}, "Owner"); err != nil {
		t.Fatal(err)
	}
	service.setHealth(app.ID, generation, Health{State: "reachable", CheckedAt: time.Now().UTC()})
	if health := service.Snapshot().Apps[0].Health; health.State != "unchecked" {
		t.Fatalf("old configuration result became current: %+v", health)
	}
}

func TestUpdatedAppStartsANewProbeFlight(t *testing.T) {
	oldEntered, releaseOld := make(chan struct{}), make(chan struct{})
	oldServer := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		close(oldEntered)
		<-releaseOld
		response.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer oldServer.Close()
	newRequests := make(chan struct{}, 1)
	newServer := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		newRequests <- struct{}{}
		response.WriteHeader(http.StatusNoContent)
	}))
	defer newServer.Close()
	app := testApp(1, oldServer.URL, true)
	service, _ := serviceForTest(t, testBoard(app))
	prober := NewProber(service, 30*time.Second, nil)
	oldResult := make(chan probeResult, 1)
	go sendProbeResult(prober, context.Background(), app.ID, oldResult)
	<-oldEntered
	newURL := newServer.URL
	if _, _, err := service.Update(context.Background(), app.ID, UpdateInput{URL: &newURL, ExpectedVersion: 1}, "Owner"); err != nil {
		t.Fatal(err)
	}
	newHealth, err := prober.ProbeOne(context.Background(), app.ID)
	if err != nil || newHealth.State != "reachable" {
		t.Fatalf("updated app check = %+v, %v", newHealth, err)
	}
	select {
	case <-newRequests:
	default:
		t.Fatal("updated app did not check the new target")
	}
	close(releaseOld)
	<-oldResult
	if health := service.Snapshot().Apps[0].Health; health.State != "reachable" {
		t.Fatalf("old flight replaced new result: %+v", health)
	}
}

func TestDisabledAppDoesNotJoinItsOldFlight(t *testing.T) {
	entered, release := make(chan struct{}), make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		close(entered)
		<-release
		response.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	app := testApp(1, server.URL, true)
	service, _ := serviceForTest(t, testBoard(app))
	prober := NewProber(service, 30*time.Second, nil)
	result := make(chan probeResult, 1)
	go sendProbeResult(prober, context.Background(), app.ID, result)
	<-entered
	disabled := false
	if _, _, err := service.Update(context.Background(), app.ID, UpdateInput{CheckEnabled: &disabled, ExpectedVersion: 1}, "Owner"); err != nil {
		t.Fatal(err)
	}
	if _, err := prober.ProbeOne(context.Background(), app.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("disabled app joined old flight: %v", err)
	}
	close(release)
	<-result
	if health := service.Snapshot().Apps[0].Health; health.State != "disabled" {
		t.Fatalf("disabled app accepted old health: %+v", health)
	}
}

type probeResult struct {
	health Health
	err    error
}

func sendProbeResult(prober *Prober, ctx context.Context, id string, target chan<- probeResult) {
	health, err := prober.ProbeOne(ctx, id)
	target <- probeResult{health: health, err: err}
}

func waitForProbeWaiters(t *testing.T, prober *Prober, id string, want int) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		prober.flightMu.Lock()
		got := 0
		for key, flight := range prober.flights {
			if key.id == id {
				got += flight.waiters
			}
		}
		prober.flightMu.Unlock()
		if got == want {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("probe never reached %d waiters", want)
}
