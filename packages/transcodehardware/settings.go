package transcodehardware

import (
	"errors"
	"fmt"

	"github.com/MikeO7/kinosail/packages/transcodepolicy"
)

// Selection contains one persisted transcoder policy choice.
type Selection struct {
	Transcoder  string `json:"transcoder,omitempty"`
	Codec       string `json:"codec,omitempty"`
	Accelerator string `json:"accelerator,omitempty"`
	ToneMap     bool   `json:"toneMap,omitempty"`
}

// SelectionSources identifies choices controlled by explicit configuration.
type SelectionSources struct{ Accelerator, Codec bool }

// NormalizeAccelerator applies Player's automatic hardware default.
func NormalizeAccelerator(accelerator string) string {
	if accelerator == "" {
		return "auto"
	}
	return accelerator
}

// NormalizeCodec applies Player's automatic codec default.
func NormalizeCodec(codec string) string {
	if codec == "" {
		return "auto"
	}
	return codec
}

// ValidAccelerator reports whether Player accepts one hardware setting.
func ValidAccelerator(accelerator string) bool {
	switch accelerator {
	case "", "none", "auto", "vaapi", "qsv", "cuda", "videotoolbox", "rkmpp", "v4l2m2m", "amf", "mf":
		return true
	default:
		return false
	}
}

// ValidateSelection validates and normalizes one untrusted settings request.
func (capabilities Capabilities) ValidateSelection(selection Selection) (Selection, error) {
	if selection.Transcoder != "automatic" && selection.Transcoder != "speed" && selection.Transcoder != "quality" {
		return Selection{}, errors.New("transcoder quality is invalid")
	}
	selection.Accelerator, selection.Codec = NormalizeAccelerator(selection.Accelerator), NormalizeCodec(selection.Codec)
	if !transcodepolicy.ValidCodec(selection.Codec) {
		return Selection{}, errors.New("video codec is invalid")
	}
	if !capabilities.SupportsCodec(selection.Codec) {
		return Selection{}, errors.New("video codec is not available on this Server")
	}
	if !ValidAccelerator(selection.Accelerator) {
		return Selection{}, errors.New("hardware accelerator is invalid")
	}
	if !capabilities.Supports(selection.Accelerator) {
		return Selection{}, errors.New("hardware accelerator is not supported on this Server")
	}
	return selection, nil
}

// ReconcileSelection repairs unavailable defaults but rejects explicit configuration.
func (capabilities Capabilities) ReconcileSelection(selection Selection, sources SelectionSources) (Selection, bool, error) {
	accelerator, codec := NormalizeAccelerator(selection.Accelerator), NormalizeCodec(selection.Codec)
	acceleratorSupported, codecSupported := capabilities.Supports(accelerator), capabilities.SupportsCodec(codec)
	if acceleratorSupported && codecSupported {
		return selection, false, nil
	}
	if !acceleratorSupported && sources.Accelerator {
		return Selection{}, false, fmt.Errorf("transcoding.accelerator %q is not supported on this Server", accelerator)
	}
	if !codecSupported && sources.Codec {
		return Selection{}, false, fmt.Errorf("transcoding.codec %q is not available on this Server", codec)
	}
	if !acceleratorSupported {
		selection.Accelerator = "auto"
	}
	if !codecSupported {
		selection.Codec = "auto"
	}
	return selection, true, nil
}

// Settings resolves one selection to the effective encoder policy.
func (capabilities Capabilities) Settings(selection Selection, requestedCodec string) (transcodepolicy.Settings, error) {
	codec := requestedCodec
	if codec == "" {
		codec = NormalizeCodec(selection.Codec)
	}
	codec = transcodepolicy.NormalizeCodec(codec)
	if !transcodepolicy.ValidCodec(codec) || !capabilities.SupportsCodec(codec) {
		return transcodepolicy.Settings{}, errors.New("video codec is not available on this Server")
	}
	accelerator, encoder := capabilities.ResolveEncoder(NormalizeAccelerator(selection.Accelerator), codec)
	if encoder == "" {
		return transcodepolicy.Settings{}, errors.New("video encoder is not available on this Server")
	}
	name := selection.Transcoder
	if name == "" {
		name = "automatic"
	}
	// Cached output follows the requested policy even if recovery changes the
	// device. HLS separately binds seek work to the encoder that made its init.
	settings := transcodepolicy.Settings{
		Name: name, Preset: "veryfast", CRF: "22", Codec: codec, Accelerator: accelerator, Encoder: encoder,
		Cache: name + ":" + codec + ":" + NormalizeAccelerator(selection.Accelerator) + ":policy=3", ToneMap: selection.ToneMap,
	}
	if operation, ok := capabilities.Backend(accelerator).operation(codec, ""); ok {
		settings.Device = operation.Device
		settings.HardwareDecode = operation.DecodeH264
	}
	if selection.ToneMap {
		settings.Cache += ":hdr"
	}
	switch selection.Transcoder {
	case "speed":
		settings.Preset, settings.CRF = "ultrafast", "26"
	case "quality":
		settings.Preset, settings.CRF = "medium", "19"
	}
	return settings, nil
}

// ColorSettings selects independently verified HDR processing. Tone mapping
// falls back to software; unverified HDR output fails without changing colors.
func (capabilities Capabilities) ColorSettings(options transcodepolicy.Settings) (transcodepolicy.Settings, error) {
	if options.OutputHDR == "" {
		return capabilities.toneMapSettings(options), nil
	}
	operation, ok := capabilities.Backend(options.Accelerator).operation(options.Codec, options.OutputHDR)
	if !ok || operation.Device != options.Device {
		return transcodepolicy.Settings{}, errors.New("the selected encoder has no verified HDR output operation")
	}
	options.Device = operation.Device
	options.HardwareDecode = false
	return options, nil
}
