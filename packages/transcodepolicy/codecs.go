// Package transcodepolicy defines Player's supported streaming codecs and
// FFmpeg argument policy for all Kinosail media applications.
package transcodepolicy

import "strings"

// Codec describes one Player-supported video codec.
type Codec struct {
	ID        string
	Name      string
	Software  []string
	Hardware  map[string]string
	Supported bool
}

var codecs = []Codec{
	{ID: "h264", Name: "AVC / H.264", Software: []string{"libx264"}, Hardware: map[string]string{"qsv": "h264_qsv", "cuda": "h264_nvenc", "vaapi": "h264_vaapi", "rkmpp": "h264_rkmpp", "v4l2m2m": "h264_v4l2m2m", "videotoolbox": "h264_videotoolbox", "amf": "h264_amf", "mf": "h264_mf"}, Supported: true},
	{ID: "hevc", Name: "HEVC / H.265", Software: []string{"libx265"}, Hardware: map[string]string{"qsv": "hevc_qsv", "cuda": "hevc_nvenc", "vaapi": "hevc_vaapi", "rkmpp": "hevc_rkmpp", "v4l2m2m": "hevc_v4l2m2m", "videotoolbox": "hevc_videotoolbox", "amf": "hevc_amf", "mf": "hevc_mf"}, Supported: true},
	{ID: "av1", Name: "AV1", Software: []string{"libsvtav1", "libaom-av1", "librav1e"}, Hardware: map[string]string{"qsv": "av1_qsv", "cuda": "av1_nvenc", "vaapi": "av1_vaapi", "amf": "av1_amf", "mf": "av1_mf"}, Supported: true},
	{ID: "vp9", Name: "VP9", Software: []string{"libvpx-vp9"}, Hardware: map[string]string{"qsv": "vp9_qsv", "vaapi": "vp9_vaapi"}, Supported: true},
	{ID: "vvc", Name: "VVC / H.266", Software: []string{"libvvenc"}},
	{ID: "av2", Name: "AV2"},
}

// Codecs returns an isolated copy of Player's codec catalog.
func Codecs() []Codec {
	result := make([]Codec, len(codecs))
	for index, codec := range codecs {
		result[index] = cloneCodec(codec)
	}
	return result
}

// NormalizeCodec resolves automatic selection to Player's compatibility codec.
func NormalizeCodec(codec string) string {
	if codec == "" || codec == "auto" {
		return "h264"
	}
	return codec
}

// ValidCodec reports whether a setting names a supported Player codec.
func ValidCodec(codec string) bool {
	if codec == "" || codec == "auto" {
		return true
	}
	definition := Definition(codec)
	return definition.ID == codec && definition.Supported
}

// Definition returns the named codec or Player's H.264 fallback.
func Definition(codec string) Codec {
	for _, definition := range codecs {
		if definition.ID == codec {
			return cloneCodec(definition)
		}
	}
	return cloneCodec(codecs[0])
}

// DefaultEncoder returns Player's preferred encoder for a backend and codec.
func DefaultEncoder(accelerator, codec string) string {
	definition := Definition(NormalizeCodec(codec))
	if accelerator == "" || accelerator == "auto" || accelerator == "none" {
		if len(definition.Software) > 0 {
			return definition.Software[0]
		}
		return ""
	}
	return definition.Hardware[accelerator]
}

// FirstEncoder selects the first exact encoder token present in FFmpeg output.
func FirstEncoder(available string, encoders []string) string {
	for _, encoder := range encoders {
		if HasCapability(available, encoder) {
			return encoder
		}
	}
	return ""
}

// EncoderCodecIDs returns codec IDs in Player preference order.
func EncoderCodecIDs(encoders map[string]string) []string {
	ids := make([]string, 0, len(encoders))
	for _, definition := range codecs {
		if encoders[definition.ID] != "" {
			ids = append(ids, definition.ID)
		}
	}
	return ids
}

// HasCapability matches one complete FFmpeg capability token.
func HasCapability(output, capability string) bool {
	for _, field := range strings.Fields(output) {
		if field == capability {
			return true
		}
	}
	return false
}

func cloneCodec(codec Codec) Codec {
	codec.Software = append([]string(nil), codec.Software...)
	if codec.Hardware != nil {
		hardware := codec.Hardware
		codec.Hardware = make(map[string]string, len(hardware))
		for backend, encoder := range hardware {
			codec.Hardware[backend] = encoder
		}
	}
	return codec
}
