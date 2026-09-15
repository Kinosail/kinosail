package transcodehardware

import (
	"strings"

	"github.com/MikeO7/kinosail/packages/transcodepolicy"
)

// HardwareFailure excludes input, storage and unrelated muxing failures. Only
// diagnostic evidence of device/frame/encoder initialization authorizes a retry.
func HardwareFailure(detail string) bool {
	detail = strings.ToLower(detail)
	for _, excluded := range []string{"no space left", "permission denied", "invalid data found", "no such file", "error opening input"} {
		if strings.Contains(detail, excluded) {
			return false
		}
	}
	for _, marker := range []string{"device setup failed", "device creation failed", "cannot load libcuda", "no capable devices", "no device available", "failed to initialise vaapi", "error initializing an internal mfx", "error while opening encoder", "failed to initialize encoder", "failed setup for format", "impossible to convert between the formats"} {
		if strings.Contains(detail, marker) {
			return true
		}
	}
	return false
}

// Recovery keeps the negotiated codec immutable. Software fallback is bounded
// to H.264; expensive formats require a newly negotiated presentation.
func (capabilities Capabilities) Recovery(options transcodepolicy.Settings) []transcodepolicy.Settings {
	var result []transcodepolicy.Settings
	if options.HardwareDecode {
		next := options
		next.HardwareDecode = false
		result = append(result, next)
	}
	for _, backend := range capabilities.Backends {
		if backend.ID == options.Accelerator || backend.ID == "none" && options.Codec != "h264" {
			continue
		}
		operation, ok := backend.operation(options.Codec, options.OutputHDR)
		if !ok {
			continue
		}
		next := options
		next.Accelerator, next.Encoder, next.Device, next.HardwareDecode = backend.ID, backend.encoders[options.Codec], operation.Device, false
		result = append(result, next)
		if len(result) == 3 {
			break
		}
	}
	return result
}

// DeviceKey shares the session budget between APIs targeting the same DRM GPU.
func DeviceKey(options transcodepolicy.Settings) string {
	if options.Accelerator == "none" {
		return ""
	}
	if options.Device == "" {
		return "system-default-gpu"
	}
	if strings.HasPrefix(options.Device, "/") {
		return options.Device
	}
	return options.Accelerator + ":" + options.Device
}
