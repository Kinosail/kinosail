package transcodepolicy

import "path/filepath"

func checkSources(directory string, options Settings) (string, [][]string) {
	transfer := checkSourceTransfer(options)
	if !options.HardwareDecode || options.DecodeInput == "" {
		return softwareCheckSource(directory, transfer)
	}
	codec, depth := DecodeInputCodec(options.DecodeInput)
	source := filepath.Join(directory, "source.mkv")
	pixelFormat := "yuv420p"
	if depth == 10 {
		pixelFormat = "yuv420p10le"
	}
	var candidates [][]string
	for _, encoder := range Definition(codec).Software {
		arguments := append(checkSourcePrefix(), decodeSourceEncoder(encoder, depth)...)
		arguments = append(arguments, "-pix_fmt", pixelFormat)
		if transfer != "" {
			arguments = append(arguments, hdrSourceArguments(transfer, pixelFormat)...)
		}
		candidates = append(candidates, append(arguments, "-c:a", "aac", source))
	}
	return source, candidates
}

func checkSourceTransfer(options Settings) string {
	hdr := ""
	switch {
	case options.HardwareToneMap != "":
		hdr = options.ToneMapInput
	case options.OutputHDR != "" && options.HardwareDecode:
		hdr = options.OutputHDR
	}
	switch hdr {
	case "hdr10":
		return "smpte2084"
	case "hlg":
		return "arib-std-b67"
	default:
		return ""
	}
}

func checkSourcePrefix() []string {
	return []string{"-hide_banner", "-loglevel", "error", "-y", "-f", "lavfi", "-i", "testsrc2=size=640x360:rate=24:duration=1", "-f", "lavfi", "-i", "sine=frequency=1000:sample_rate=48000:duration=1", "-frames:v", "24", "-shortest"}
}

func softwareCheckSource(directory, transfer string) (string, [][]string) {
	if transfer == "" {
		source := filepath.Join(directory, "source.mp4")
		return source, [][]string{append(checkSourcePrefix(), "-c:v", "libx264", "-preset", "ultrafast", "-threads", "1", "-pix_fmt", "yuv420p", "-c:a", "aac", source)}
	}
	source := filepath.Join(directory, "source.mkv")
	arguments := append(checkSourcePrefix(), "-c:v", "ffv1", "-threads", "1", "-pix_fmt", "yuv420p10le")
	arguments = append(arguments, hdrSourceArguments(transfer, "yuv420p10le")...)
	return source, [][]string{append(arguments, "-c:a", "aac", source)}
}

func hdrSourceArguments(transfer, pixelFormat string) []string {
	return []string{"-vf", "format=" + pixelFormat + ",zscale=pin=bt709:tin=bt709:min=bt709:rin=limited:p=bt2020:t=" + transfer + ":m=bt2020nc:r=limited", "-color_primaries", "bt2020", "-color_trc", transfer, "-colorspace", "bt2020nc", "-color_range", "tv"}
}

func decodeSourceEncoder(encoder string, depth int) []string {
	switch encoder {
	case "libx265":
		return []string{"-c:v", encoder, "-preset", "ultrafast", "-threads", "1"}
	case "libvpx-vp9":
		arguments := []string{"-c:v", encoder, "-deadline", "realtime", "-cpu-used", "8", "-threads", "1"}
		if depth == 10 {
			arguments = append(arguments, "-profile:v", "2")
		}
		return arguments
	case "libsvtav1":
		return []string{"-c:v", encoder, "-preset", "12"}
	case "libaom-av1":
		return []string{"-c:v", encoder, "-usage", "realtime", "-cpu-used", "8", "-threads", "1"}
	case "librav1e":
		return []string{"-c:v", encoder, "-speed", "10"}
	default:
		return []string{"-c:v", encoder, "-preset", "ultrafast", "-threads", "1"}
	}
}

func decodeInputCodec(input string) string {
	codec, _ := DecodeInputCodec(input)
	return codec
}
