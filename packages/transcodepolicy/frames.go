package transcodepolicy

import "strings"

func frameArguments(options Settings, width string) ([]string, string) {
	if input, filter := hardwareToneMapFrames(options, width); filter != "" {
		return input, filter
	}
	if options.HardwareDecode && !options.SoftwareFilters && !options.ToneMap && !options.Deinterlace {
		if input := acceleratedInput(options); len(input) > 0 {
			return input, acceleratedScaler(options, width)
		}
	}
	filters := softwareFrameFilters(options, width)
	input := []string{}
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

func acceleratedInput(options Settings) []string {
	switch options.Accelerator {
	case "cuda":
		input := []string{"-hwaccel", "cuda", "-hwaccel_output_format", "cuda"}
		if options.Device != "" {
			input = append(input, "-hwaccel_device", options.Device)
		}
		return input
	case "qsv":
		return append(qsvDevice(options.Device), "-hwaccel", "qsv", "-hwaccel_device", "kino", "-hwaccel_output_format", "qsv")
	case "vaapi":
		return []string{"-hwaccel", "vaapi", "-hwaccel_device", renderDevice(options.Device), "-hwaccel_output_format", "vaapi"}
	case "rkmpp":
		return []string{"-init_hw_device", "rkmpp=rk", "-hwaccel", "rkmpp", "-hwaccel_output_format", "drm_prime"}
	case "videotoolbox":
		return []string{"-hwaccel", "videotoolbox", "-hwaccel_output_format", "videotoolbox"}
	}
	return nil
}

func acceleratedScaler(options Settings, width string) string {
	size := "=w=" + width + ":h=-2"
	switch options.Accelerator {
	case "cuda":
		return "scale_cuda" + size + ":format=" + cudaFormat(options)
	case "qsv":
		return "scale_qsv" + size + ":format=" + surfaceFormat(options)
	case "vaapi":
		return "scale_vaapi" + size + ":format=" + surfaceFormat(options)
	case "rkmpp":
		return "scale_rkrga" + size + ":format=" + surfaceFormat(options)
	case "videotoolbox":
		return "scale_vt" + size
	}
	return ""
}

func HardwareDecodeInput(accelerator string) bool {
	return acceleratedInput(Settings{Accelerator: accelerator}) != nil
}

func qsvDevice(device string) []string {
	if strings.HasPrefix(device, "/") {
		return []string{"-init_hw_device", "vaapi=kino_va:" + device, "-init_hw_device", "qsv=kino@kino_va", "-filter_hw_device", "kino"}
	}
	return []string{"-init_hw_device", "qsv=kino", "-filter_hw_device", "kino"}
}

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

func cudaFormat(options Settings) string {
	if PixelFormat(options) == "yuv420p10le" {
		return "p010le"
	}
	return "yuv420p"
}

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
		filters = append(filters, softwareToneMapFilters()...)
	}
	filters = append(filters, "scale=w="+width+":h='trunc(ow/dar/2)*2'", "setsar=1", "format="+PixelFormat(options))
	return filters
}

func softwareToneMapFilters() []string {
	return append([]string{"zscale=t=linear:npl=100,format=gbrpf32le,zscale=p=bt709,tonemap=hable:desat=0,zscale=t=bt709:m=bt709:r=tv"}, removeHDRMetadata()...)
}
