package main

import (
	"bytes"
	"context"
	"log/slog"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/MikeO7/kinosail/packages/appcli"
)

type warmupKey struct{}

type warmupResult struct {
	listening bool
	lifecycle bool
	err       error
}

func TestServeWarmsDefaultLanguageViewsAfterListening(t *testing.T) {
	t.Setenv("KINOSAIL_DATA_DIR", t.TempDir())
	configured, err := loadConfiguration()
	if err != nil {
		t.Fatal(err)
	}
	listener, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	results := make(chan warmupResult, 2)
	previous := warmLocalizedViews
	warmLocalizedViews = func(ctx context.Context) error {
		connection, dialErr := (&net.Dialer{Timeout: 5 * time.Second}).DialContext(ctx, "tcp", listener.Addr().String())
		if dialErr == nil {
			_ = connection.Close()
		}
		err := previous(ctx)
		results <- warmupResult{listening: dialErr == nil, lifecycle: ctx.Value(warmupKey{}) == "serve", err: err}
		return err
	}
	t.Cleanup(func() { warmLocalizedViews = previous })
	ctx, cancel := context.WithCancel(context.WithValue(t.Context(), warmupKey{}, "serve"))
	defer cancel()
	httpServer := newConfiguredHTTPServerWithAccess(ctx, configured, nil, nil)
	select {
	case result := <-results:
		t.Fatalf("warm-up ran before the listener served: %+v", result)
	case <-time.After(100 * time.Millisecond):
	}
	served := make(chan error, 1)
	go func() { served <- httpServer.Serve(listener) }()
	defer func() {
		_ = httpServer.Close()
		<-served
	}()
	select {
	case result := <-results:
		if !result.listening || !result.lifecycle || result.err != nil {
			t.Fatalf("warm-up = %+v", result)
		}
	case <-time.After(60 * time.Second):
		t.Fatal("serve path did not warm localized views")
	}
	select {
	case result := <-results:
		t.Fatalf("warm-up ran twice: %+v", result)
	case <-time.After(100 * time.Millisecond):
	}
}

func TestCommandsWithoutHTMLDoNotWarmLocalizedViews(t *testing.T) {
	t.Setenv("KINOSAIL_DATA_DIR", t.TempDir())
	t.Setenv("KINOSAIL_LISTEN", "127.0.0.1:1")
	warmed := 0
	previous := warmLocalizedViews
	warmLocalizedViews = func(context.Context) error {
		warmed++
		return nil
	}
	logger := slog.Default()
	t.Cleanup(func() {
		warmLocalizedViews = previous
		slog.SetDefault(logger)
	})
	for _, args := range [][]string{{"version"}, {"healthcheck"}, {"backup"}, {"config", "validate"}} {
		var output bytes.Buffer
		appcli.Execute(args, strings.NewReader(""), &output, func(string) string { return "" }, nil, application())
	}
	if warmed != 0 {
		t.Fatalf("command-line paths warmed localized views %d times", warmed)
	}
}
