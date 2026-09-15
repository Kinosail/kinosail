package transcodehardware

import (
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/MikeO7/kinosail/packages/transcodepolicy"
)

func TestSelectionStateValidatesBeforePersistence(t *testing.T) {
	initial := Selection{Transcoder: "speed", Codec: "auto", Accelerator: "none"}
	value, check, saves := initial, transcodepolicy.CheckResult{Status: "passed"}, 0
	state := SelectionState{Lock: &sync.RWMutex{}, Value: &value, Check: &check, Save: func() error { saves++; return nil }}
	managed := errors.New("managed")
	if err := state.Set(settingsCapabilities(), Selection{Transcoder: "automatic"}, func(...string) error { return managed }); !errors.Is(err, managed) {
		t.Fatalf("managed selection error = %v", err)
	}
	if err := state.Set(settingsCapabilities(), Selection{Transcoder: strings.Repeat("x", 16<<10)}, func(...string) error { return nil }); err == nil {
		t.Fatal("oversized selection was accepted")
	}
	if saves != 0 || value != initial || check.Status != "passed" {
		t.Fatalf("rejected selection caused effects: saves=%d value=%#v check=%#v", saves, value, check)
	}
}

func TestSelectionStatePersistsBeforeCommit(t *testing.T) {
	initial := Selection{Transcoder: "speed", Codec: "auto", Accelerator: "none"}
	value, check := initial, transcodepolicy.CheckResult{Status: "passed"}
	saveErr := errors.New("save failed")
	state := SelectionState{Lock: &sync.RWMutex{}, Value: &value, Check: &check, Save: func() error { return saveErr }}
	requested := Selection{Transcoder: "automatic", ToneMap: true}
	if err := state.Set(settingsCapabilities(), requested, func(...string) error { return nil }); !errors.Is(err, saveErr) || value != initial || check.Status != "passed" {
		t.Fatalf("failed save committed state: err=%v value=%#v check=%#v", err, value, check)
	}
	var saved Selection
	state.Save = func() error { saved = value; return nil }
	if err := state.Set(settingsCapabilities(), requested, func(keys ...string) error {
		if len(keys) != 4 {
			t.Fatalf("editable keys = %v", keys)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	want := Selection{Transcoder: "automatic", Codec: "auto", Accelerator: "auto", ToneMap: true}
	if value != want || saved != want || check != (transcodepolicy.CheckResult{}) {
		t.Fatalf("selection=%#v saved=%#v check=%#v", value, saved, check)
	}
}

func TestSelectionStateReconcilesAndReads(t *testing.T) { //nolint:cyclop // One test covers each reconciliation outcome and read adapter.
	capabilities := settingsCapabilities()
	value, check, saves := Selection{Transcoder: "quality", Codec: "auto", Accelerator: "qsv", ToneMap: true}, transcodepolicy.CheckResult{Status: "passed"}, 0
	state := SelectionState{Lock: &sync.RWMutex{}, Value: &value, Check: &check, Save: func() error { saves++; return nil }}
	if err := state.Reconcile(capabilities, SelectionSources{}); err != nil || saves != 0 {
		t.Fatalf("supported reconcile = %v, saves=%d", err, saves)
	}
	value.Accelerator = "vaapi"
	if err := state.Reconcile(capabilities, SelectionSources{Accelerator: true}); err == nil || saves != 0 || value.Accelerator != "vaapi" {
		t.Fatalf("explicit reconcile = %v, saves=%d, value=%#v", err, saves, value)
	}
	saveErr := errors.New("save failed")
	state.Save = func() error { return saveErr }
	if err := state.Reconcile(capabilities, SelectionSources{}); !errors.Is(err, saveErr) || value.Accelerator != "vaapi" {
		t.Fatalf("failed reconcile = %v, value=%#v", err, value)
	}
	state.Save = func() error { saves++; return nil }
	if err := state.Reconcile(capabilities, SelectionSources{}); err != nil || value.Accelerator != "auto" || saves != 1 || check.Status != "passed" {
		t.Fatalf("reconcile = %v, saves=%d, value=%#v, check=%#v", err, saves, value, check)
	}
	settings, err := state.Settings(capabilities, "h264")
	if err != nil || settings.Codec != "h264" || state.PreferredCodec(capabilities, []string{"h264"}) != "h264" {
		t.Fatalf("settings=%#v err=%v", settings, err)
	}
}

func TestSelectionJSONAndConfigurationSources(t *testing.T) {
	wrapped := struct{ Selection }{Selection{Transcoder: "quality", Codec: "hevc", Accelerator: "qsv", ToneMap: true}}
	encoded, err := json.Marshal(wrapped)
	if err != nil || string(encoded) != `{"transcoder":"quality","codec":"hevc","accelerator":"qsv","toneMap":true}` {
		t.Fatalf("encoded selection = %s, %v", encoded, err)
	}
	var decoded struct{ Selection }
	if err = json.Unmarshal(encoded, &decoded); err != nil || decoded != wrapped {
		t.Fatalf("decoded selection = %#v, %v", decoded, err)
	}
	if ConfiguredSources("", "default") != (SelectionSources{}) || ConfiguredSources("yaml", "gui") != (SelectionSources{Accelerator: true, Codec: true}) {
		t.Fatal("configuration source classification changed")
	}
}
