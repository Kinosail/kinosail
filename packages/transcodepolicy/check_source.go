package transcodepolicy

import "path/filepath"

func checkSource(directory string, options Settings) (string, []string) {
	source := filepath.Join(directory, "source.mkv")
	args := []string{"-hide_banner", "-loglevel", "error", "-y", "-f", "lavfi", "-i", "testsrc2=size=640x360:rate=24:duration=1", "-f", "lavfi", "-i", "sine=frequency=1000:sample_rate=48000:duration=1", "-frames:v", "24", "-shortest"}
	transfer := "smpte2084"
	if options.ToneMapInput == "hlg" {
		transfer = "arib-std-b67"
	}
	hdr := []string{"-vf", "format=yuv420p10le,zscale=pin=bt709:tin=bt709:min=bt709:rin=limited:p=bt2020:t=" + transfer + ":m=bt2020nc:r=limited"}
	tags := []string{"-color_primaries", "bt2020", "-color_trc", transfer, "-colorspace", "bt2020nc", "-color_range", "tv"}
	switch {
	case options.HardwareToneMap == "" && options.DecodeInput != "" && options.DecodeInput != "h264":
		args = append(args, decodeSourceArguments(options.DecodeInput)...)
	case options.HardwareToneMap == "":
		source = filepath.Join(directory, "source.mp4")
		args = append(args, "-c:v", "libx264", "-preset", "ultrafast", "-threads", "1", "-pix_fmt", "yuv420p")
	case options.DecodeInput == "hevc10":
		args = append(append(append(args, hdr...), decodeSourceArguments("hevc10")...), tags...)
	default:
		args = append(append(append(args, hdr...), "-c:v", "ffv1", "-threads", "1", "-pix_fmt", "yuv420p10le"), tags...)
	}
	return source, append(args, "-c:a", "aac", source)
}

func validDecodeInput(input string) bool {
	switch input {
	case "", "h264", "hevc", "hevc10", "av1":
		return true
	default:
		return false
	}
}

func decodeSourceArguments(input string) []string {
	switch input {
	case "hevc":
		return []string{"-c:v", "libx265", "-preset", "ultrafast", "-x265-params", "log-level=error", "-pix_fmt", "yuv420p"}
	case "hevc10":
		return []string{"-c:v", "libx265", "-preset", "ultrafast", "-x265-params", "log-level=error", "-pix_fmt", "yuv420p10le"}
	default:
		return []string{"-c:v", "libsvtav1", "-preset", "12", "-pix_fmt", "yuv420p10le"}
	}
}
