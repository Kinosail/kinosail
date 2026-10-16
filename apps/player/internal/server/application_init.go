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

func closeApplicationDatabase(lifecycle context.Context, stateDB *database.Store, managed bool) func(func()) {
	if !managed || stateDB == nil {
		return func(func()) {}
	}
	var mu sync.Mutex
	var drains []func()
	go func() {
		<-lifecycle.Done()
		mu.Lock()
		defer mu.Unlock()
		for _, drain := range drains {
			drain()
		}
		_ = stateDB.Close()
	}()
	return func(drain func()) {
		mu.Lock()
		defer mu.Unlock()
		drains = append(drains, drain)
	}
}

func startAccessFlush(lifecycle context.Context, managed bool, profiles *profileStore, beforeClose func(func())) {
	if !managed {
		return
	}
	flushed := make(chan struct{})
	go func() {
		defer close(flushed)
		profiles.runAccessFlush(lifecycle, accessFlushInterval)
	}()
	beforeClose(func() { <-flushed })
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
	settings.hardware = probeHardware(config.Lifecycle, config.FFmpeg, config.HardwareDevices, config.ProbeHardware, config.HardwareOS, config.HardwareArch)
	if settings.reconcileHardwareSelection() != nil {
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
