package server

import (
	"encoding/json"
	"io"
	"os"
	"time"
)

func (registry *subtitleProviderHealthRegistry) load() {
	file, err := os.Open(registry.path)
	if err != nil {
		return
	}
	defer file.Close()
	decoder := json.NewDecoder(io.LimitReader(file, 64<<10))
	decoder.DisallowUnknownFields()
	var disk subtitleProviderHealthDiskState
	if decoder.Decode(&disk) != nil || decoder.Decode(&struct{}{}) != io.EOF || disk.Version != 1 || len(disk.Providers) > 3 {
		return
	}
	now := time.Now()
	for name, entry := range disk.Providers {
		if !oneOf(name, "SubDL", "OpenSubtitles", "SubSource") || entry.LastSuccess < 0 || entry.LastSuccess > now.Add(24*time.Hour).Unix() || entry.Reset < 0 || entry.Reset > now.Add(8*24*time.Hour).Unix() || entry.NextRetry < 0 || entry.NextRetry > now.Add(8*24*time.Hour).Unix() || entry.Failures < 0 || entry.Failures > 1000 || !oneOf(entry.LastError, "", "Credentials rejected", "Rate limit reached", "Provider connection failed", "Provider unavailable") || entry.Remaining != nil && (*entry.Remaining < 0 || *entry.Remaining > 1_000_000_000) {
			return
		}
		registry.states[name] = subtitleProviderHealthState{lastSuccess: subtitleTimeFromUnix(entry.LastSuccess), reset: subtitleTimeFromUnix(entry.Reset), nextRetry: subtitleTimeFromUnix(entry.NextRetry), remaining: entry.Remaining, lastError: entry.LastError, failures: entry.Failures}
	}
}

func (registry *subtitleProviderHealthRegistry) saveLocked() {
	if registry.path == "" {
		return
	}
	disk := subtitleProviderHealthDiskState{Version: 1, Providers: make(map[string]subtitleProviderHealthEntry, len(registry.states))}
	for name, state := range registry.states {
		disk.Providers[name] = subtitleProviderHealthEntry{subtitleUnix(state.lastSuccess), subtitleUnix(state.reset), subtitleUnix(state.nextRetry), state.remaining, state.lastError, state.failures}
	}
	now := registry.now()
	if sameSubtitleProviderHealth(registry.saved, disk) && now.Sub(registry.savedAt) < subtitleProviderHealthRefresh && !now.Before(registry.savedAt) {
		return
	}
	if saveDurableJSON(registry.path, disk) == nil {
		registry.saved, registry.savedAt = disk, now
	}
}

func sameSubtitleProviderHealth(saved, current subtitleProviderHealthDiskState) bool {
	if saved.Providers == nil || len(saved.Providers) != len(current.Providers) {
		return false
	}
	for name, entry := range current.Providers {
		previous, found := saved.Providers[name]
		if !found || previous.Reset != entry.Reset || previous.NextRetry != entry.NextRetry || previous.LastError != entry.LastError || previous.Failures != entry.Failures || (previous.Remaining == nil) != (entry.Remaining == nil) {
			return false
		}
	}
	return true
}

func subtitleUnix(value time.Time) int64 {
	if value.IsZero() {
		return 0
	}
	return value.Unix()
}

func subtitleTimeFromUnix(value int64) time.Time {
	if value == 0 {
		return time.Time{}
	}
	return time.Unix(value, 0)
}
