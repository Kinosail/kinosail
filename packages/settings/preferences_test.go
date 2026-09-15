package settings

import (
	"errors"
	"reflect"
	"sync"
	"testing"
)

func TestChangePlaybackValidationAndEffects(t *testing.T) { //nolint:cyclop // One table proves every validation branch has no persistence effect.
	t.Parallel()
	managed, failed := errors.New("managed"), errors.New("failed")
	tests := []struct {
		name       string
		input      Playback
		editErr    error
		persistErr error
		want       Playback
		wantErr    string
	}{
		{"managed", Playback{}, managed, nil, Playback{}, "managed"},
		{"mode", Playback{PlaybackMode: "other"}, nil, nil, Playback{}, "playback mode is invalid"},
		{"subtitles", Playback{PlaybackMode: "direct", Subtitles: "sometimes"}, nil, nil, Playback{}, "subtitle preference is invalid"},
		{"markers", Playback{PlaybackMode: "compatible", Subtitles: "off", AutoSkip: []string{"unknown"}}, nil, nil, Playback{}, "unknown automatic skip marker type"},
		{"persist", Playback{PlaybackMode: "automatic", Subtitles: "on"}, nil, failed, Playback{PlaybackMode: "automatic", Subtitles: "on"}, "failed"},
		{"defaults", Playback{PlaybackMode: "automatic", Autoplay: true, AutoSkip: []string{"intro", "intro", "credits"}}, nil, nil, Playback{PlaybackMode: "automatic", Subtitles: "on", Autoplay: true, AutoSkip: []string{"intro", "credits"}}, ""},
		{"nil markers", Playback{PlaybackMode: "direct", Subtitles: "off"}, nil, nil, Playback{PlaybackMode: "direct", Subtitles: "off"}, ""},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			edits, persists := 0, 0
			var saved Playback
			err := ChangePlayback(test.input, func(keys ...string) error {
				edits++
				if !reflect.DeepEqual(keys, playbackKeys) {
					t.Fatalf("editable keys = %v", keys)
				}
				return test.editErr
			}, func(input Playback) error { persists++; saved = input; return test.persistErr })
			if edits != 1 || persists != boolInt(test.editErr == nil && test.wantErr != "playback mode is invalid" && test.wantErr != "subtitle preference is invalid" && test.wantErr != "unknown automatic skip marker type") || (test.wantErr == "" && err != nil) || (test.wantErr != "" && (err == nil || err.Error() != test.wantErr)) || persists == 1 && !reflect.DeepEqual(saved, test.want) {
				t.Fatalf("result err=%v edits=%d persists=%d saved=%#v", err, edits, persists, saved)
			}
		})
	}
}

func TestPersistAndRead(t *testing.T) {
	t.Parallel()
	mutex, current, committed := &sync.RWMutex{}, 1, 0
	failed := errors.New("save")
	err := Persist(mutex, func() int { return current }, func(value *int) { *value = 2 }, func(value int) error {
		if value != 2 {
			t.Fatalf("saved = %d", value)
		}
		return failed
	}, func(int) { committed++ })
	if !errors.Is(err, failed) || committed != 0 {
		t.Fatalf("failure = %v commits=%d", err, committed)
	}
	err = Persist(mutex, func() int { return current }, func(value *int) { *value = 3 }, func(int) error { return nil }, func(value int) { current, committed = value, committed+1 })
	if err != nil || current != 3 || committed != 1 || Read(mutex, func() int { return current }) != 3 {
		t.Fatalf("success = %v current=%d commits=%d", err, current, committed)
	}
}

func TestPlaybackAndScanDefaults(t *testing.T) { //nolint:cyclop // One assertion group proves the related persisted playback defaults.
	t.Parallel()
	if PlaybackMode("") != "automatic" || PlaybackMode("direct") != "direct" || !(Playback{}).SubtitlesDefault() || (Playback{Subtitles: "off"}).SubtitlesDefault() || !(Playback{}).AutoplayDefault() || !(Playback{PlaybackMode: "direct", Autoplay: true}).AutoplayDefault() || (Playback{PlaybackMode: "direct"}).AutoplayDefault() {
		t.Fatal("playback defaults are invalid")
	}
	defaults := (Playback{}).AutoSkipDefault()
	copyValue := (Playback{AutoSkip: []string{"intro"}}).AutoSkipDefault()
	copyValue[0] = "changed"
	if len(defaults) == 0 || copyValue[0] != "changed" {
		t.Fatalf("marker defaults = %v copied=%v", defaults, copyValue)
	}
	original := []string{"intro"}
	copyValue = (Playback{AutoSkip: original}).AutoSkipDefault()
	copyValue[0] = "changed"
	if original[0] != "intro" {
		t.Fatal("auto-skip result aliases persisted state")
	}
	current := Playback{AutoSkip: []string{"credits"}}
	if got := (Playback{PlaybackMode: "direct"}).Merge(current); !reflect.DeepEqual(got.AutoSkip, current.AutoSkip) {
		t.Fatalf("omitted markers = %v", got.AutoSkip)
	}
	if got := (Playback{AutoSkip: []string{}}).Merge(current); got.AutoSkip == nil || len(got.AutoSkip) != 0 {
		t.Fatalf("explicit empty markers = %#v", got.AutoSkip)
	}
	for input, want := range map[string]string{"": "default", "default": "default", "other": "default", "off": "off", "5m": "5m", "15m": "15m", "1h": "1h"} {
		if got := ScanFrequency(input); got != want {
			t.Fatalf("frequency %q = %q", input, got)
		}
	}
}

func TestChangeScanFrequencyValidationAndEffects(t *testing.T) { //nolint:cyclop,gocognit // The compact matrix proves every validation and persistence outcome.
	t.Parallel()
	managed := errors.New("managed")
	for _, test := range []struct {
		frequency  string
		editErr    error
		persistErr error
		wantCalls  int
	}{
		{"invalid", nil, nil, 0}, {"default", managed, nil, 0}, {"off", nil, errors.New("save"), 1}, {"5m", nil, nil, 1}, {"15m", nil, nil, 1}, {"1h", nil, nil, 1},
	} {
		calls := 0
		err := ChangeScanFrequency(test.frequency, func(keys ...string) error {
			if !reflect.DeepEqual(keys, []string{"scanning.frequency"}) {
				t.Fatalf("editable keys = %v", keys)
			}
			return test.editErr
		}, func(value string) error {
			calls++
			if value != test.frequency {
				t.Fatalf("persisted %q", value)
			}
			return test.persistErr
		})
		if calls != test.wantCalls || (test.frequency == "5m" || test.frequency == "15m" || test.frequency == "1h") && err != nil || test.wantCalls == 0 && err == nil || test.persistErr != nil && !errors.Is(err, test.persistErr) {
			t.Fatalf("frequency %q: err=%v calls=%d", test.frequency, err, calls)
		}
	}
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}
