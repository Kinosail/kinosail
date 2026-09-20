package transcodepolicy

import "strings"

// ToneMapMethods lists device frame paths to verify, in preference order.
// A compiled filter is not evidence that the device can execute it.
func ToneMapMethods(backend, device string) []string {
	switch backend {
	case "cuda":
		return []string{"cuda"}
	case "qsv":
		if strings.HasPrefix(device, "/") {
			return []string{"qsv", "vulkan", "opencl"}
		}
		return []string{"qsv", "d3d11"}
	case "vaapi":
		return []string{"vaapi", "vulkan", "opencl"}
	case "videotoolbox":
		return []string{"videotoolbox"}
	case "amf":
		return []string{"d3d11"}
	case "rkmpp":
		return []string{"vulkan", "opencl"}
	}
	return nil
}

func validToneMapMethod(options Settings) bool {
	if options.ToneMapInput == "hlg" && (options.HardwareToneMap == "qsv" || options.HardwareToneMap == "vaapi") {
		return false
	}
	for _, method := range ToneMapMethods(options.Accelerator, options.Device) {
		if method == options.HardwareToneMap {
			return true
		}
	}
	return false
}

// hardwareToneMapFrames keeps HDR conversion on the GPU. Returning SDR frames
// to the existing software seam preserves rotation, subtitle burn-in and skip
// filters without treating an encode smoke check as HDR decode evidence.
func hardwareToneMapFrames(options Settings, width string) ([]string, string) {
	if !options.ToneMap || options.DisableHardwareToneMap || options.OutputHDR != "" || !validToneMapMethod(options) || options.ToneMapInput != "hdr10" && options.ToneMapInput != "hlg" {
		return nil, ""
	}
	input, filter := toneMapDeviceFilter(options)
	if !ToneMapSoftwareFrames(options) {
		filter += "," + strings.Join(removeHDRMetadata(), ",")
		scaler := map[string]string{"cuda": "scale_cuda", "qsv": "scale_qsv", "vaapi": "scale_vaapi"}[options.Accelerator]
		return input, filter + "," + scaler + "=w=" + width + ":h=-2:format=nv12,setsar=1"
	}
	filter += ",hwdownload,format=nv12"
	// Strip HDR side data after conversion; output pixels and tags are SDR.
	filter += "," + strings.Join(removeHDRMetadata(), ",")
	software := options
	software.ToneMap = false
	filters := softwareFrameFilters(software, width)
	if options.Accelerator == "qsv" || options.Accelerator == "vaapi" {
		filters[len(filters)-1] = "format=nv12"
	}
	filter += "," + strings.Join(filters, ",")
	// Derived OpenCL processing returns to the selected VA-API device;
	// QSV encoding derives its upload device from that same DRM node.
	switch options.Accelerator {
	case "qsv", "vaapi":
		if options.HardwareToneMap == "opencl" || options.HardwareToneMap == "vulkan" || options.HardwareToneMap == "d3d11" {
			derive := options.Accelerator
			filter += ",hwupload=derive_device=" + derive
		} else {
			filter += ",hwupload=extra_hw_frames=32"
		}
	}
	return input, filter
}

func removeHDRMetadata() []string {
	var filters []string
	for _, kind := range []string{"MASTERING_DISPLAY_METADATA", "CONTENT_LIGHT_LEVEL", "DYNAMIC_HDR_PLUS", "DOVI_RPU_BUFFER", "DOVI_METADATA"} {
		filters = append(filters, "sidedata=mode=delete:type="+kind)
	}
	return filters
}

// ToneMapSoftwareFrames identifies paths that need SDR frames on the CPU.
func ToneMapSoftwareFrames(options Settings) bool {
	return options.SoftwareFilters || options.Deinterlace || options.HardwareToneMap == "opencl" || options.HardwareToneMap == "vulkan" || options.HardwareToneMap == "d3d11" || options.Accelerator != "cuda" && options.Accelerator != "qsv" && options.Accelerator != "vaapi"
}

func toneMapDeviceFilter(options Settings) ([]string, string) {
	var input []string
	filter := ""
	color := ":format=nv12:p=bt709:t=bt709:m=bt709:r=tv"
	switch options.HardwareToneMap {
	case "cuda":
		device := options.Device
		if device == "" {
			device = "0"
		}
		input = []string{"-init_hw_device", "cuda=kino:" + device, "-filter_hw_device", "kino"}
		filter = "format=p010le,hwupload,tonemap_cuda=tonemap=bt2390" + color
	case "vaapi":
		input = []string{"-init_hw_device", "vaapi=kino:" + renderDevice(options.Device), "-filter_hw_device", "kino"}
		filter = "format=p010le,hwupload,tonemap_vaapi=format=nv12:p=bt709:t=bt709:m=bt709"
	case "qsv":
		input = qsvDevice(options.Device)
		filter = "format=p010le,hwupload=extra_hw_frames=32,vpp_qsv=tonemap=1:format=nv12:out_range=tv"
	case "opencl":
		input, filter = openCLToneMapFilter(options, color)
	case "vulkan":
		deviceType := "vaapi"
		if options.Accelerator == "rkmpp" {
			deviceType = "drm"
		}
		input = []string{"-init_hw_device", deviceType + "=kino_base:" + renderDevice(options.Device), "-init_hw_device", "vulkan=kino@kino_base", "-filter_hw_device", "kino_base"}
		filter = "format=p010le,hwupload=derive_device=vulkan,libplacebo=format=nv12:tonemapping=bt.2390:color_primaries=bt709:color_trc=bt709:colorspace=bt709:range=tv"
	case "videotoolbox":
		input = []string{"-init_hw_device", "videotoolbox=kino", "-filter_hw_device", "kino"}
		filter = "format=p010le,hwupload,tonemap_videotoolbox=tonemap=bt2390" + color
	case "d3d11":
		input = []string{"-init_hw_device", "d3d11va=kino", "-filter_hw_device", "kino"}
		filter = "format=p010le,hwupload,tonemap_d3d11=tonemap=bt2390" + color
	}
	return input, filter
}

func openCLToneMapFilter(options Settings, color string) ([]string, string) {
	var input []string
	var filter string
	if options.Accelerator == "vaapi" || options.Accelerator == "qsv" {
		input = []string{"-init_hw_device", "vaapi=kino_va:" + renderDevice(options.Device), "-init_hw_device", "opencl=kino_cl@kino_va", "-filter_hw_device", "kino_va"}
	} else {
		input = []string{"-init_hw_device", "opencl=kino:,device_type=gpu", "-filter_hw_device", "kino"}
	}
	filter = "format=p010le,hwupload,tonemap_opencl=tonemap=bt2390" + color
	if options.Accelerator == "vaapi" || options.Accelerator == "qsv" {
		filter = "format=p010le,hwupload,hwmap=derive_device=opencl,tonemap_opencl=tonemap=bt2390" + color + ",hwmap=derive_device=vaapi:reverse=1"
	}
	return input, filter
}
