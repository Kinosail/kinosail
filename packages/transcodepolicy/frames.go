package transcodepolicy

import "strings"

func frameArguments(options Settings, width string) ([]string, string) {
	if options.HardwareDecode && !options.SoftwareFilters && !options.ToneMap && !options.Deinterlace {
		if input, filter := acceleratedFrames(options, width); len(input) > 0 {
			return input, filter
		}
	}
	filters := softwareFrameFilters(options, width)
	input := []string{}
	// VA-API and QSV encoders consume hardware frames. Upload only after all
	// software operations, including subtitle rendering, have finished.
	switch options.Accelerator {
	case "vaapi":
		input = []string{"-init_hw_device", "vaapi=kino:" + renderDevice(options.Device), "-filter_hw_device", "kino"}
		filters[len(filters)-1] = "format=" + surfaceFormat(options)
		filters = append(filters, "hwupload")
	case "qsv":
		input = qsvDevice(options.Device)
		filters[len(filters)-1] = "format=" + surfaceFormat(options)
		filters = append(filters, "hwupload=extra_hw_frames=32")
	}
	return input, strings.Join(filters, ",")
}

func acceleratedFrames(options Settings, width string) ([]string, string) {
	format := surfaceFormat(options)
	switch options.Accelerator {
	case "cuda":
		input := []string{"-hwaccel", "cuda", "-hwaccel_output_format", "cuda"}
		if options.Device != "" {
			input = append(input, "-hwaccel_device", options.Device)
		}
		return input, "scale_cuda=w=" + width + ":h=-2:format=" + PixelFormat(options)
	case "qsv":
		input := qsvDevice(options.Device)
		input = append(input, "-hwaccel", "qsv", "-hwaccel_device", "kino", "-hwaccel_output_format", "qsv")
		return input, "scale_qsv=w=" + width + ":h=-2:format=" + format
	case "vaapi":
		return []string{"-hwaccel", "vaapi", "-hwaccel_device", renderDevice(options.Device), "-hwaccel_output_format", "vaapi"}, "scale_vaapi=w=" + width + ":h=-2:format=" + format
	case "rkmpp":
		return []string{"-init_hw_device", "rkmpp=rk", "-hwaccel", "rkmpp", "-hwaccel_output_format", "drm_prime"}, "scale_rkrga=w=" + width + ":h=-2:format=" + format
	}
	return nil, ""
}

func qsvDevice(device string) []string {
	if strings.HasPrefix(device, "/") {
		return []string{"-init_hw_device", "vaapi=kino_va:" + device, "-init_hw_device", "qsv=kino@kino_va", "-filter_hw_device", "kino"}
	}
	return []string{"-init_hw_device", "qsv=kino", "-filter_hw_device", "kino"}
}

// PixelFormat is the software representation of the negotiated output color.
func PixelFormat(options Settings) string {
	if options.OutputHDR == "hdr10" || options.OutputHDR == "hlg" {
		return "yuv420p10le"
	}
	return "yuv420p"
}

func surfaceFormat(options Settings) string {
	if PixelFormat(options) == "yuv420p10le" {
		return "p010le"
	}
	return "nv12"
}

// ColorArguments keeps encoded pixels and their color declarations consistent.
func ColorArguments(options Settings) []string {
	if options.ToneMap {
		return []string{"-color_primaries", "bt709", "-color_trc", "bt709", "-colorspace", "bt709", "-color_range", "tv"}
	}
	if options.OutputHDR != "" {
		transfer := "smpte2084"
		if options.OutputHDR == "hlg" {
			transfer = "arib-std-b67"
		}
		return []string{"-color_primaries", "bt2020", "-color_trc", transfer, "-colorspace", "bt2020nc", "-color_range", "tv"}
	}
	return nil
}

func softwareFrameFilters(options Settings, width string) []string {
	filters := []string{}
	if options.Deinterlace {
		filters = append(filters, "bwdif=mode=send_frame:parity=auto:deint=interlaced")
	}
	if options.ToneMap {
		filters = append(filters, "zscale=t=linear:npl=100,format=gbrpf32le,zscale=p=bt709,tonemap=hable:desat=0,zscale=t=bt709:m=bt709:r=tv")
		for _, kind := range []string{"MASTERING_DISPLAY_METADATA", "CONTENT_LIGHT_LEVEL", "DYNAMIC_HDR_PLUS", "DOVI_RPU_BUFFER", "DOVI_METADATA"} {
			filters = append(filters, "sidedata=mode=delete:type="+kind)
		}
	}
	filters = append(filters, "scale=w="+width+":h='trunc(ow/dar/2)*2'", "setsar=1", "format="+PixelFormat(options))
	return filters
}
