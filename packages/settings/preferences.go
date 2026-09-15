// Package settings owns shared Kinosail settings validation and HTTP operations.
package settings

import (
	"errors"
	"sync"

	"github.com/MikeO7/kinosail/packages/markers"
)

var playbackKeys = []string{"playback.mode", "playback.autoplay", "playback.subtitles", "playback.auto_skip"}

// Playback contains the settings changed by the playback form.
type Playback struct {
	PlaybackMode string   `json:"playbackMode,omitempty"`
	Autoplay     bool     `json:"autoplay,omitempty"`
	AutoSkip     []string `json:"autoSkip"`
	Subtitles    string   `json:"subtitles,omitempty"`
}

// SubtitlesDefault reports whether subtitles should start enabled.
func (playback Playback) SubtitlesDefault() bool { return playback.Subtitles != "off" }

// AutoplayDefault reports whether playback should continue automatically.
func (playback Playback) AutoplayDefault() bool {
	return playback.PlaybackMode == "" || playback.Autoplay
}

// AutoSkipDefault returns a detached marker list and applies its default.
func (playback Playback) AutoSkipDefault() []string {
	if playback.AutoSkip == nil {
		return markers.Types()
	}
	return append([]string(nil), playback.AutoSkip...)
}

// Merge preserves markers when an API update omits that optional field.
func (playback Playback) Merge(current Playback) Playback {
	if playback.AutoSkip == nil {
		playback.AutoSkip = current.AutoSkip
	}
	return playback
}

// Persist applies one change to a copied app settings value and commits it after a successful save.
func Persist[T any](lock sync.Locker, current func() T, apply func(*T), save func(T) error, commit func(T)) error {
	lock.Lock()
	defer lock.Unlock()
	value := current()
	apply(&value)
	if err := save(value); err != nil {
		return err
	}
	commit(value)
	return nil
}

// ChangePlayback validates one complete playback change before persisting it.
func ChangePlayback(input Playback, editable func(...string) error, persist func(Playback) error) error {
	if err := editable(playbackKeys...); err != nil {
		return err
	}
	if input.PlaybackMode != "automatic" && input.PlaybackMode != "direct" && input.PlaybackMode != "compatible" {
		return errors.New("playback mode is invalid")
	}
	if input.Subtitles == "" {
		input.Subtitles = "on"
	}
	if input.Subtitles != "on" && input.Subtitles != "off" {
		return errors.New("subtitle preference is invalid")
	}
	if input.AutoSkip != nil {
		normalized, err := markers.NormalizeAutoSkip(input.AutoSkip)
		if err != nil {
			return err
		}
		input.AutoSkip = normalized
	}
	return persist(input)
}

// PlaybackMode applies the default playback mode.
func PlaybackMode(mode string) string {
	if mode == "" {
		return "automatic"
	}
	return mode
}

// ChangeScanFrequency validates one scan schedule before persisting it.
func ChangeScanFrequency(frequency string, editable func(...string) error, persist func(string) error) error {
	if err := editable("scanning.frequency"); err != nil {
		return err
	}
	if frequency != "default" && frequency != "off" && frequency != "5m" && frequency != "15m" && frequency != "1h" {
		return errors.New("scan frequency is invalid")
	}
	return persist(frequency)
}

// ScanFrequency applies the default scan schedule.
func ScanFrequency(frequency string) string {
	switch frequency {
	case "off", "5m", "15m", "1h":
		return frequency
	default:
		return "default"
	}
}

type readLocker interface {
	RLock()
	RUnlock()
}

// Read locks a shared app settings value while deriving one result.
func Read[T any](lock readLocker, value func() T) T {
	lock.RLock()
	defer lock.RUnlock()
	return value()
}
