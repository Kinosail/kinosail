package settings

import (
	"errors"
	"sync"

	"github.com/MikeO7/kinosail/packages/markers"
)

var playbackKeys = []string{"playback.mode", "playback.autoplay", "playback.subtitles", "playback.auto_skip"}

type Playback struct {
	PlaybackMode string   `json:"playbackMode,omitempty"`
	Autoplay     bool     `json:"autoplay,omitempty"`
	AutoSkip     []string `json:"autoSkip"`
	Subtitles    string   `json:"subtitles,omitempty"`
}

func (playback Playback) SubtitlesDefault() bool { return playback.Subtitles != "off" }

func (playback Playback) AutoplayDefault() bool {
	return playback.PlaybackMode == "" || playback.Autoplay
}

func (playback Playback) AutoSkipDefault() []string {
	if playback.AutoSkip == nil {
		return markers.Types()
	}
	return append([]string(nil), playback.AutoSkip...)
}

func (playback Playback) Merge(current Playback) Playback {
	if playback.AutoSkip == nil {
		playback.AutoSkip = current.AutoSkip
	}
	return playback
}

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

func PlaybackMode(mode string) string {
	if mode == "" {
		return "automatic"
	}
	return mode
}

func ChangeScanFrequency(frequency string, editable func(...string) error, persist func(string) error) error {
	if err := editable("scanning.frequency"); err != nil {
		return err
	}
	if frequency != "default" && frequency != "off" && frequency != "5m" && frequency != "15m" && frequency != "1h" {
		return errors.New("scan frequency is invalid")
	}
	return persist(frequency)
}

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

func Read[T any](lock readLocker, value func() T) T {
	lock.RLock()
	defer lock.RUnlock()
	return value()
}
