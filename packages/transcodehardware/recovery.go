package transcodehardware

import (
	"strings"

	"github.com/MikeO7/kinosail/packages/transcodepolicy"
)

func HardwareFailure(detail string) bool {
	detail = strings.ToLower(detail)
	for _, excluded := range []string{"no space left", "permission denied", "invalid data found", "no such file", "error opening input"} {
		if strings.Contains(detail, excluded) {
			return false
		}
	}
	for _, marker := range []string{"device setup failed", "device creation failed", "cannot load libcuda", "no capable devices", "no device available", "failed to initialise vaapi", "error initializing an internal mfx", "error while opening encoder", "failed to initialize encoder", "failed setup for format", "impossible to convert between the formats", "tonemap_cuda", "tonemap_opencl", "tonemap_vaapi", "tonemap_videotoolbox", "tonemap_d3d11", "vpp_qsv", "libplacebo", "failed to create vulkan", "failed to initialise opencl", "failed to initialize opencl", "parsed_hwupload", "parsed_hwmap", "parsed_scale_cuda", "parsed_scale_qsv", "parsed_scale_vaapi", "parsed_scale_vt", "parsed_scale_rkrga", "parsed_tonemap", "parsed_vpp_qsv", "hwaccel initialisation returned error", "hardware accelerator failed to decode picture", "no decoder surfaces left", "failed to get hw surface", "failed to create a videotoolbox"} {
		if strings.Contains(detail, marker) {
			return true
		}
	}
	return false
}

func (capabilities Capabilities) Recovery(options transcodepolicy.Settings) []transcodepolicy.Settings {
	var result []transcodepolicy.Settings
	if options.HardwareToneMap != "" && options.HardwareDecode {
		next := options
		next.HardwareDecode = false
		result = append(result, next)
	}
	if options.HardwareToneMap != "" {
		next := options
		next.HardwareToneMap, next.HardwareDecode, next.DisableHardwareToneMap = "", false, true
		result = append(result, next)
	} else if options.HardwareDecode {
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
		next.HardwareToneMap, next.DisableHardwareToneMap = "", true
		next.Accelerator, next.Encoder, next.Device, next.HardwareDecode, next.HardwareDecodeInputs = backend.ID, backend.encoders[options.Codec], operation.Device, false, nil
		result = append(result, next)
		if len(result) == 3 {
			break
		}
	}
	return result
}

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
