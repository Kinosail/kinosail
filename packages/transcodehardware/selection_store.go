package transcodehardware

import (
	"sync"

	"github.com/MikeO7/kinosail/packages/transcodepolicy"
)

// SelectionState adapts transcoder policy to an application's persisted settings.
type SelectionState struct {
	Lock  *sync.RWMutex
	Value *Selection
	Save  func() error
	Check *transcodepolicy.CheckResult
}

// ConfiguredSources identifies choices supplied outside built-in defaults.
func ConfiguredSources(accelerator, codec string) SelectionSources {
	return SelectionSources{Accelerator: accelerator != "" && accelerator != "default", Codec: codec != "" && codec != "default"}
}

// Set validates and persists one untrusted transcoder selection.
func (state SelectionState) Set(capabilities Capabilities, selection Selection, editable func(...string) error) error {
	if err := editable("transcoding.quality", "transcoding.codec", "transcoding.accelerator", "transcoding.tone_map"); err != nil {
		return err
	}
	validated, err := capabilities.ValidateSelection(selection)
	if err != nil {
		return err
	}
	state.Lock.Lock()
	defer state.Lock.Unlock()
	return state.persist(validated, true)
}

// Reconcile repairs unavailable defaults and persists the result.
func (state SelectionState) Reconcile(capabilities Capabilities, sources SelectionSources) error {
	state.Lock.Lock()
	defer state.Lock.Unlock()
	selection, changed, err := capabilities.ReconcileSelection(*state.Value, sources)
	if err != nil || !changed {
		return err
	}
	return state.persist(selection, false)
}

// Settings resolves the current selection for one output codec.
func (state SelectionState) Settings(capabilities Capabilities, codec string) (transcodepolicy.Settings, error) {
	return capabilities.Settings(state.current(), codec)
}

// PreferredCodec selects the best client codec for the current selection.
func (state SelectionState) PreferredCodec(capabilities Capabilities, client []string) string {
	selection := state.current()
	return capabilities.PreferredCodec(NormalizeCodec(selection.Codec), NormalizeAccelerator(selection.Accelerator), client)
}

func (state SelectionState) current() Selection {
	state.Lock.RLock()
	defer state.Lock.RUnlock()
	return *state.Value
}

func (state SelectionState) persist(selection Selection, reset bool) error {
	previous := *state.Value
	*state.Value = selection
	if err := state.Save(); err != nil {
		*state.Value = previous
		return err
	}
	if reset {
		*state.Check = transcodepolicy.CheckResult{}
	}
	return nil
}

// CheckSettings permits an explicit diagnostic to recheck discovered operations
// without making those unverified operations available to normal playback.
func (state SelectionState) CheckSettings(capabilities Capabilities) (transcodepolicy.Settings, error) {
	candidates := capabilities
	candidates.verification = nil
	candidates.Backends = append([]Backend(nil), capabilities.Backends...)
	for index := range candidates.Backends {
		backend := &candidates.Backends[index]
		backend.verification = nil
		backend.Usable = backend.Supported && backend.Detected
	}
	return candidates.Settings(state.current(), "")
}

// CheckCoordinator binds persisted selection and hardware evidence to the
// shared explicit-check lifecycle.
func (state SelectionState) CheckCoordinator(ffmpeg string, capabilities Capabilities) transcodepolicy.CheckCoordinator {
	return transcodepolicy.NewCheckCoordinator(transcodepolicy.CheckCoordinatorDependencies{
		Lock: state.Lock, Result: state.Check, FFmpeg: ffmpeg,
		Resolve: func() (transcodepolicy.Settings, error) {
			return state.CheckSettings(capabilities)
		},
		Pending: func() transcodepolicy.Settings {
			settings, _ := state.Settings(capabilities, "")
			return settings
		},
		BackendName:    func(accelerator string) string { return capabilities.Backend(accelerator).Name },
		RecordHardware: capabilities.RecordCheck,
	})
}
