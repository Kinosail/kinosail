package transcodepolicy

import "path/filepath"

func checkSource(directory string, options Settings) (string, []string) {
	source := filepath.Join(directory, "source.mp4")
	args := []string{"-hide_banner", "-loglevel", "error", "-y", "-f", "lavfi", "-i", "testsrc2=size=640x360:rate=24:duration=1", "-f", "lavfi", "-i", "sine=frequency=1000:sample_rate=48000:duration=1", "-frames:v", "24", "-shortest"}
	if options.HardwareToneMap == "" {
		args = append(args, "-c:v", "libx264", "-preset", "ultrafast", "-threads", "1", "-pix_fmt", "yuv420p")
	} else {
		source = filepath.Join(directory, "source.mkv")
		transfer := "smpte2084"
		if options.ToneMapInput == "hlg" {
			transfer = "arib-std-b67"
		}
		// Convert actual SDR test pixels into a 10-bit HDR signal; do not merely
		// attach HDR tags to the ordinary 8-bit encoder fixture.
		args = append(args, "-vf", "format=yuv420p10le,zscale=pin=bt709:tin=bt709:min=bt709:rin=limited:p=bt2020:t="+transfer+":m=bt2020nc:r=limited", "-c:v", "ffv1", "-threads", "1", "-pix_fmt", "yuv420p10le", "-color_primaries", "bt2020", "-color_trc", transfer, "-colorspace", "bt2020nc", "-color_range", "tv")
	}
	return source, append(args, "-c:a", "aac", source)
}
