package server

import (
	"context"
	"log/slog"
	"path/filepath"
	"time"

	"github.com/MikeO7/kinosail/packages/transcodehardware"
)

type (
	hardwareBackend      = transcodehardware.Backend
	hardwareCapabilities = transcodehardware.Capabilities
)

func (store *settingsStore) hardware() hardwareCapabilities {
	if current := store.hardwareState.Load(); current != nil {
		return *current
	}
	return hardwareCapabilities{Selected: "none"}
}

func (store *settingsStore) setHardware(capabilities hardwareCapabilities) {
	store.hardwareState.Store(&capabilities)
}

func (store *settingsStore) startHardwareProbe(config Config) error {
	options := transcodehardware.ProbeOptions{Application: "Kinosail Player", FFmpeg: config.FFmpeg, Devices: config.HardwareDevices, Enabled: config.ProbeHardware, GOOS: config.HardwareOS, GOARCH: config.HardwareArch}
	if !options.Enabled || options.FFmpeg == "" {
		store.setHardware(transcodehardware.Probe(config.Lifecycle, options))
		return store.reconcileHardwareSelection()
	}
	cache := ""
	if config.CacheDir != "" {
		cache = filepath.Join(config.CacheDir, "hardware-probe.json")
	}
	if cached, ok := transcodehardware.CachedProbe(cache, options); ok {
		store.setHardware(cached)
		slog.Info("hardware probe restored", "operation", "hardware_probe", "outcome", "cache_hit", "selected", cached.Selected)
		return store.reconcileHardwareSelection()
	}
	store.setHardware(transcodehardware.Provisional(options))
	if err := store.reconcileHardwareSelection(); err != nil {
		return err
	}
	go store.completeHardwareProbe(config.Lifecycle, options, cache)
	return nil
}

func (store *settingsStore) completeHardwareProbe(ctx context.Context, options transcodehardware.ProbeOptions, cache string) {
	started := time.Now()
	capabilities := transcodehardware.Probe(ctx, options)
	if ctx.Err() != nil {
		return
	}
	if cache != "" {
		if err := transcodehardware.StoreProbe(cache, options, capabilities); err != nil {
			slog.Warn("hardware probe cache unavailable", "operation", "hardware_probe", "outcome", "cache_write_failed")
		}
	}
	store.setHardware(capabilities)
	if err := store.reconcileHardwareSelection(); err != nil {
		slog.Error("transcoder configuration is unavailable", "operation", "hardware_probe", "outcome", "configuration_rejected", "selected", capabilities.Selected)
		return
	}
	slog.Info("hardware probe completed", "operation", "hardware_probe", "outcome", "verified", "selected", capabilities.Selected, "duration_ms", time.Since(started).Milliseconds())
}
