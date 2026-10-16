package transcodepolicy

import "slices"

var decodeInputs = []string{"h264", "hevc10", "hevc", "av110", "av1", "vp9", "vp910"}

func DecodeInputs() []string {
	return append([]string(nil), decodeInputs...)
}

func ValidDecodeInput(input string) bool {
	return slices.Contains(decodeInputs, input)
}

func DecodeInput(codec string, bitDepth int) string {
	if bitDepth != 0 && bitDepth != 8 && bitDepth != 10 {
		return ""
	}
	tenBit := bitDepth == 10
	switch codec {
	case "h264":
		if tenBit {
			return ""
		}
		return "h264"
	case "hevc", "vp9", "av1":
		if tenBit {
			return codec + "10"
		}
		return codec
	default:
		return ""
	}
}

func DecodeInputCodec(input string) (string, int) {
	switch input {
	case "h264", "hevc", "av1", "vp9":
		return input, 8
	case "hevc10", "av110", "vp910":
		return input[:len(input)-2], 10
	default:
		return "", 0
	}
}

func HardwareDecodeAvailable(options Settings) bool {
	return options.DecodeInput != "" && slices.Contains(options.HardwareDecodeInputs, options.DecodeInput)
}
