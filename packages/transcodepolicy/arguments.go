package transcodepolicy

type Settings struct {
	Name                   string
	Preset                 string
	CRF                    string
	Codec                  string
	Accelerator            string
	Encoder                string
	Cache                  string
	Device                 string
	HardwareToneMap        string
	ToneMapInput           string
	DisableHardwareToneMap bool
	ToneMap                bool
	HardwareDecode         bool
	SoftwareFilters        bool
	Deinterlace            bool
	OutputHDR              string
}

func VideoArguments(options Settings, width string) ([]string, []string) {
	codec := NormalizeCodec(options.Codec)
	encoderName := options.Encoder
	if encoderName == "" {
		encoderName = DefaultEncoder(options.Accelerator, codec)
	}
	input, filter := frameArguments(options, width)
	encoder := encoderArguments(encoderName, options)
	if codec == "hevc" {
		encoder = append(encoder, "-tag:v", "hvc1")
	}
	encoder = append(encoder, "-fpsmax", "60")
	encoder = append(encoder, ColorArguments(options)...)
	return input, append([]string{"-vf", filter}, encoder...)
}

func encoderArguments(name string, options Settings) []string {
	switch options.Accelerator {
	case "vaapi":
		return []string{"-c:v", name, "-qp", options.CRF}
	case "qsv":
		return []string{"-c:v", name, "-global_quality", options.CRF}
	case "cuda":
		return cudaEncoderArguments(name, options)
	case "videotoolbox":
		return []string{"-c:v", name, "-q:v", options.CRF, "-allow_sw", "0"}
	case "rkmpp":
		return []string{"-c:v", name, "-rc_mode", "CQP", "-qp_init", options.CRF}
	case "amf":
		return []string{"-c:v", name, "-quality", "balanced", "-qp_i", options.CRF, "-qp_p", options.CRF}
	case "mf":
		return []string{"-c:v", name, "-hw_encoding", "1"}
	case "v4l2m2m":
		return []string{"-c:v", name}
	default:
		return softwareVideoEncoderArguments(name, options)
	}
}

func CompatibilityArguments(codec string) []string {
	if NormalizeCodec(codec) == "h264" {
		return []string{"-profile:v", "high", "-level:v", "4.2"}
	}
	return nil
}

func softwareVideoEncoderArguments(encoder string, options Settings) []string {
	switch encoder {
	case "libsvtav1":
		preset := map[string]string{"ultrafast": "10", "veryfast": "8", "medium": "6"}[options.Preset]
		return []string{"-c:v", encoder, "-preset", preset, "-crf", options.CRF}
	case "libaom-av1":
		return []string{"-c:v", encoder, "-usage", "realtime", "-cpu-used", "8", "-row-mt", "1", "-tile-columns", "2", "-crf", options.CRF}
	case "librav1e":
		return []string{"-c:v", encoder, "-speed", "8", "-qp", options.CRF}
	case "libvpx-vp9":
		return []string{"-c:v", encoder, "-deadline", "realtime", "-cpu-used", "6", "-row-mt", "1", "-tile-columns", "2", "-crf", options.CRF, "-b:v", "0"}
	default:
		return []string{"-c:v", encoder, "-preset", options.Preset, "-crf", options.CRF}
	}
}

func renderDevice(device string) string {
	if device == "" {
		return "/dev/dri/renderD128"
	}
	return device
}

func cudaEncoderArguments(name string, options Settings) []string {
	args := []string{"-c:v", name, "-cq", options.CRF, "-preset", "p4"}
	if options.Device != "" {
		args = append(args, "-gpu", options.Device)
	}
	return args
}
