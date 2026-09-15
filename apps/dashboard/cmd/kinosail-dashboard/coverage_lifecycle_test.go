package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"testing"
	"time"
)

func TestCoverageMainVersion(t *testing.T) {
	arguments, logger := os.Args, slog.Default()
	t.Cleanup(func() { os.Args = arguments; slog.SetDefault(logger) })
	os.Args = []string{"kinosail-dashboard", "version"}
	main()
}

func TestCoverageApplicationLifecycleCancellation(t *testing.T) {
	config := runtimeConfig{listen: "127.0.0.1:0", dataDir: t.TempDir(), publicURL: "http://localhost:38400", probeInterval: time.Hour}
	app, err := openApplication(t.Context(), config)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = app.store.Close() })
	if app.board.Snapshot().Title != "Home" || app.auth.Configured() {
		t.Fatal("new application state is invalid")
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if err := serve(ctx, app, config); !errors.Is(err, http.ErrServerClosed) {
		t.Fatalf("canceled serve = %v", err)
	}
}
