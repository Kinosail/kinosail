package dashboard

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"
)

func TestCoverageProbeFailureExplanations(t *testing.T) {
	for _, test := range []struct {
		err  error
		want string
	}{
		{&url.Error{Op: "Get", URL: "http://local", Err: context.Canceled}, "Check timed out"},
		{&net.OpError{Op: "read", Err: context.DeadlineExceeded}, "Check timed out"},
		{&net.OpError{Op: "dial", Err: errors.New("refused")}, "Service did not accept a connection"},
		{&net.OpError{Op: "read", Err: errors.New("closed")}, "Service could not be reached"},
	} {
		if got := explainProbeError(test.err); got != test.want {
			t.Fatalf("explanation = %q, want %q", got, test.want)
		}
	}
	if !forbiddenIP(nil) {
		t.Fatal("nil IP allowed")
	}
}

func TestCoverageProbeRejectsInvalidDestinations(t *testing.T) {
	lookup := func(context.Context, string) ([]net.IPAddr, error) { return nil, errors.New("unresolved") }
	for _, address := range []string{"invalid", "metadata:80", "unknown.test:80"} {
		if connection, err := safeDialContext(t.Context(), "tcp", address, nil, lookup); err == nil || connection != nil {
			t.Fatalf("destination %q accepted", address)
		}
	}
}

func TestCoverageProbeInvalidHealthRequests(t *testing.T) {
	service, _ := serviceForTest(t, testBoard())
	prober := NewProber(service, 0, nil)
	if prober.interval != 30*time.Second {
		t.Fatalf("default interval = %v", prober.interval)
	}
	if got := prober.check(t.Context(), "invalid"); got.State != "unavailable" || got.Explanation != "Health address is invalid" {
		t.Fatalf("invalid health = %+v", got)
	}
	if got := prober.check(t.Context(), " http://127.0.0.1:1 "); got.State != "unavailable" || got.Explanation != "Request could not be created" {
		t.Fatalf("untrimmed health = %+v", got)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if got := prober.check(ctx, "http://127.0.0.1:1"); got.State != "unavailable" || got.Explanation != "Check timed out" {
		t.Fatalf("canceled health = %+v", got)
	}
}

func TestCoverageProbeRunsUntilCancellation(t *testing.T) {
	requests := make(chan struct{}, 8)
	provider := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		requests <- struct{}{}
		writer.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(provider.Close)
	service, _ := serviceForTest(t, testBoard(testApp(1, provider.URL, true)))
	prober := NewProber(service, time.Hour, nil)
	prober.interval = time.Millisecond
	ctx, cancel := context.WithCancel(t.Context())
	t.Cleanup(cancel)
	done := make(chan struct{})
	go func() { prober.Run(ctx); close(done) }()
	for range 2 {
		select {
		case <-requests:
		case <-time.After(5 * time.Second):
			t.Fatal("periodic probe did not run")
		}
	}
	cancel()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("probe ignored cancellation")
	}
}

func TestCoverageSlowProbeResponse(t *testing.T) {
	provider := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		time.Sleep(slowThreshold + 20*time.Millisecond)
		writer.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(provider.Close)
	service, _ := serviceForTest(t, testBoard())
	prober := NewProber(service, time.Hour, nil)
	if got := prober.check(t.Context(), provider.URL); got.State != "slow" || got.LatencyMS < slowThreshold.Milliseconds() {
		t.Fatalf("slow health = %+v", got)
	}
}
