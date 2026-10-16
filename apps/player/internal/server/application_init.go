package server

import (
	"context"
	"log/slog"
	"sync"

	"github.com/MikeO7/kinosail-player/internal/database"
	"github.com/MikeO7/kinosail/packages/updatecontrol"
)

func prepareApplicationConfig(config *Config) (context.Context, bool) {
	lifecycle := config.Lifecycle
	if config.Lifecycle == nil {
		config.Lifecycle = context.Background()
	}
	if config.FFmpeg == "" {
		config.FFmpeg = "ffmpeg"
	}
	if config.FPCalc == "" {
		config.FPCalc = "fpcalc"
	}
	return lifecycle, lifecycle != nil
}

type databaseShutdown struct {
	mu     sync.Mutex
	closed bool
	hooks  []func()
}

func (shutdown *databaseShutdown) beforeClose(hook func()) {
	shutdown.mu.Lock()
	if !shutdown.closed {
		shutdown.hooks = append(shutdown.hooks, hook)
		shutdown.mu.Unlock()
		return
	}
	shutdown.mu.Unlock()
	hook()
}

func (shutdown *databaseShutdown) run() {
	shutdown.mu.Lock()
	hooks := shutdown.hooks
	shutdown.closed, shutdown.hooks = true, nil
	shutdown.mu.Unlock()
	for _, hook := range hooks {
		hook()
	}
}

func closeApplicationDatabase(lifecycle context.Context, stateDB *database.Store, managed bool) *databaseShutdown {
	shutdown := new(databaseShutdown)
	if !managed || stateDB == nil {
		return shutdown
	}
	go func() {
		<-lifecycle.Done()
		shutdown.run()
		_ = stateDB.Close()
	}()
	return shutdown
}

func initializeApplicationSettings(config Config, stateDB *database.Store, managed bool) (*settingsStore, *updateChecker, string) {
	settings := newSettingsStore(config.MediaDir, config.DataDir, config.DLNAURL, stateDB, config.Configuration)
	if config.TrustedHTTPSCheck != nil {
		settings.trustedHTTPSCheck = config.TrustedHTTPSCheck
	}
	if settings.err != nil {
		return nil, nil, "application state is unavailable"
	}
	updateManager, err := updatecontrol.New(stateDB, updateReleasePolicy())
	if err != nil {
		return nil, nil, "application state is unavailable"
	}
	updates := newUpdateChecker(settings, updateManager)
	if managed {
		updates.Schedule(config.Lifecycle)
	}
	settings.ffmpeg = config.FFmpeg
	if settings.startHardwareProbe(config) != nil {
		return nil, nil, "transcoder configuration is unavailable"
	}
	return settings, updates, ""
}

func startApplicationMCPHost(config Config, managed bool, adapter *mcpAdapter, profiles *profileStore) {
	if !managed || config.DataDir == "" {
		return
	}
	if err := startMCPStdioHost(config.Lifecycle, config.DataDir, adapter, profiles); err != nil {
		slog.Error("MCP STDIO unavailable", "error", err)
	}
}

func disableInternetForBrokenProfiles(config Config, auth *authentication) {
	if auth.profiles.err != nil && config.InternetAccess != nil {
		_ = config.InternetAccess.Kill()
	}
}
