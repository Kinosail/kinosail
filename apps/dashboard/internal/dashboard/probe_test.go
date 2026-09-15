package dashboard

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestProbeAllSkipsDisabledChecks(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		response.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	service, _ := serviceForTest(t, testBoard(
		testApp(1, server.URL, true),
		testApp(2, server.URL+"/disabled", false),
	))
	prober := NewProber(service, 30*time.Second, nil)
	batch := prober.ProbeAll(context.Background())
	if batch.Requested != 1 || batch.Completed != 1 || batch.Canceled {
		t.Fatalf("unexpected batch: %+v", batch)
	}
	if _, err := prober.ProbeOne(context.Background(), service.Snapshot().Apps[1].ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("disabled check should not be probeable, got %v", err)
	}
	snapshot := service.Snapshot()
	if snapshot.Apps[0].Health.State != "reachable" || snapshot.Apps[1].Health.State != "disabled" {
		t.Fatalf("unexpected health states: %+v", snapshot.Apps)
	}
	if snapshot.Summary.Total != 2 || snapshot.Summary.Reachable != 1 || snapshot.Summary.Disabled != 1 {
		t.Fatalf("unexpected summary: %+v", snapshot.Summary)
	}
}

func TestProbeClassifiesHTTPStatusesWithoutFollowingRedirects(t *testing.T) {
	handler := http.NewServeMux()
	handler.HandleFunc("/ok", func(response http.ResponseWriter, _ *http.Request) { response.WriteHeader(http.StatusOK) })
	handler.HandleFunc("/auth", func(response http.ResponseWriter, _ *http.Request) { response.WriteHeader(http.StatusUnauthorized) })
	handler.HandleFunc("/redirect", func(response http.ResponseWriter, request *http.Request) {
		http.Redirect(response, request, "/ok", http.StatusFound)
	})
	handler.HandleFunc("/error", func(response http.ResponseWriter, _ *http.Request) {
		response.WriteHeader(http.StatusServiceUnavailable)
	})
	server := httptest.NewServer(handler)
	defer server.Close()
	prober := NewProber(nil, 30*time.Second, nil)

	tests := []struct {
		path        string
		wantState   string
		wantStatus  int
		explanation string
	}{
		{path: "/ok", wantState: "reachable", wantStatus: http.StatusOK},
		{path: "/auth", wantState: "reachable", wantStatus: http.StatusUnauthorized},
		{path: "/redirect", wantState: "reachable", wantStatus: http.StatusFound, explanation: "HTTP 302 responded; redirect not followed"},
		{path: "/error", wantState: "degraded", wantStatus: http.StatusServiceUnavailable},
	}
	for _, test := range tests {
		t.Run(test.path, func(t *testing.T) {
			health := prober.check(context.Background(), server.URL+test.path)
			if health.State != test.wantState || health.HTTPStatus != test.wantStatus {
				t.Fatalf("unexpected observation: %+v", health)
			}
			if test.explanation != "" && health.Explanation != test.explanation {
				t.Fatalf("got explanation %q, want %q", health.Explanation, test.explanation)
			}
			if health.CheckedAt.IsZero() {
				t.Fatal("successful HTTP observation needs a timestamp")
			}
		})
	}
}

func TestInFlightProbeCannotRestoreHealthAfterUpdate(t *testing.T) {
	entered := make(chan struct{})
	release := make(chan struct{})
	defer func() {
		select {
		case <-release:
		default:
			close(release)
		}
	}()
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		close(entered)
		<-release
		response.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	app := testApp(1, server.URL, true)
	service, _ := serviceForTest(t, testBoard(app))
	prober := NewProber(service, 30*time.Second, nil)
	result := make(chan error, 1)
	go func() {
		_, err := prober.ProbeOne(context.Background(), app.ID)
		result <- err
	}()
	select {
	case <-entered:
	case <-time.After(2 * time.Second):
		t.Fatal("probe did not reach server")
	}

	name := "Renamed application"
	if _, _, err := service.Update(context.Background(), app.ID, UpdateInput{Name: &name, ExpectedVersion: 1}, "Owner"); err != nil {
		t.Fatal(err)
	}
	close(release)
	if err := <-result; err != nil {
		t.Fatal(err)
	}
	health := service.Snapshot().Apps[0].Health
	if health.State != "unchecked" || health.Explanation != "Not checked yet" {
		t.Fatalf("stale in-flight result became current: %+v", health)
	}
}

func TestProbeDestinationSafetyBlocksMetadataAndLinkLocalAddresses(t *testing.T) {
	if !forbiddenHost("metadata.google.internal.") || !forbiddenHost("metadata") {
		t.Fatal("metadata host aliases must be blocked")
	}
	for _, address := range []string{"169.254.169.254", "::ffff:169.254.169.254", "100.100.100.200", "224.0.0.1", "::"} {
		if !forbiddenIP(net.ParseIP(address)) {
			t.Fatalf("address %s must be blocked", address)
		}
	}
	for _, address := range []string{"127.0.0.1", "10.0.0.1", "192.168.1.1"} {
		if forbiddenIP(net.ParseIP(address)) {
			t.Fatalf("local address %s should be allowed", address)
		}
	}
}

func TestProbeDestinationSafetyRequiresPublicHostAllowlist(t *testing.T) {
	publicLookup := func(context.Context, string) ([]net.IPAddr, error) {
		return []net.IPAddr{{IP: net.ParseIP("203.0.113.10")}}, nil
	}
	_, err := safeDialContext(context.Background(), "tcp", "public.example:443", nil, publicLookup)
	if err == nil || err.Error() != "public probe destination is not allowed" {
		t.Fatalf("unexpected public-host rejection: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	allowedDial := newProbeDialer([]string{"Public.Example."}, publicLookup)
	_, err = allowedDial(ctx, "tcp", "public.example:443")
	if err == nil || err.Error() == "public probe destination is not allowed" {
		t.Fatalf("allowlisted host did not advance to the network dial: %v", err)
	}
	_, err = allowedDial(context.Background(), "tcp", "child.public.example:443")
	if err == nil || err.Error() != "public probe destination is not allowed" {
		t.Fatalf("allowlist must require an exact host match: %v", err)
	}
}

func TestProbeDestinationSafetyRejectsMixedResolution(t *testing.T) {
	lookup := func(context.Context, string) ([]net.IPAddr, error) {
		return []net.IPAddr{
			{IP: net.ParseIP("203.0.113.10")},
			{IP: net.ParseIP("169.254.169.254")},
		}, nil
	}
	_, err := safeDialContext(context.Background(), "tcp", "service.example:80", map[string]bool{"service.example": true}, lookup)
	if err == nil || err.Error() != "probe destination is blocked" {
		t.Fatalf("mixed resolution must fail closed, got %v", err)
	}
}

func TestQueuedProbeAllHonorsCancellation(t *testing.T) {
	entered, release := make(chan struct{}), make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		close(entered)
		<-release
		response.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()
	service, _ := serviceForTest(t, testBoard(testApp(1, server.URL, true)))
	prober := NewProber(service, 30*time.Second, nil)
	first := make(chan ProbeBatch, 1)
	go func() { first <- prober.ProbeAll(context.Background()) }()
	select {
	case <-entered:
	case <-time.After(2 * time.Second):
		t.Fatal("first board check did not start")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	started := time.Now()
	queued := prober.ProbeAll(ctx)
	if time.Since(started) > 100*time.Millisecond || !queued.Canceled || queued.Requested != 1 || queued.Completed != 0 {
		t.Fatalf("canceled queued check = %+v after %s", queued, time.Since(started))
	}
	close(release)
	<-first
}

func TestProbeOneCoalescesConcurrentChecksForOneApp(t *testing.T) {
	entered, release := make(chan struct{}), make(chan struct{})
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		close(entered)
		<-release
		response.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	app := testApp(1, server.URL, true)
	service, _ := serviceForTest(t, testBoard(app))
	prober := NewProber(service, 30*time.Second, nil)
	results := make(chan error, 4)
	go func() { _, err := prober.ProbeOne(context.Background(), app.ID); results <- err }()
	<-entered
	for range 3 {
		go func() { _, err := prober.ProbeOne(context.Background(), app.ID); results <- err }()
	}
	time.Sleep(40 * time.Millisecond)
	if requests.Load() != 1 {
		t.Fatalf("concurrent checks made %d network requests", requests.Load())
	}
	close(release)
	for range 4 {
		if err := <-results; err != nil {
			t.Fatal(err)
		}
	}
}
