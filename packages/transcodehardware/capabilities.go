package transcodehardware

import (
	"slices"

	"github.com/MikeO7/kinosail/packages/transcodepolicy"
)

// Backend describes one FFmpeg hardware or software encoding backend.
type Backend struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	Encoder      string   `json:"encoder"`
	Codecs       []string `json:"codecs"`
	Device       string   `json:"-"`
	Supported    bool     `json:"supported"`
	Detected     bool     `json:"detected"`
	Usable       bool     `json:"usable"`
	Relevant     bool     `json:"relevant"`
	Status       string   `json:"status"`
	Reason       string   `json:"reason,omitempty"`
	Action       string   `json:"action,omitempty"`
	encoders     map[string]string
	Operations   []Operation `json:"operations,omitempty"`
	verification *verification
}

// EncoderFor returns the backend's discovered encoder for a codec.
func (backend Backend) EncoderFor(codec string) string {
	if backend.verification != nil {
		if _, ok := backend.operation(codec, ""); !ok {
			return ""
		}
	}
	return backend.encoders[codec]
}

// Codec describes the presented readiness of one video codec.
type Codec transcodepolicy.Capability

// Capabilities is one coherent snapshot of codec and hardware readiness.
type Capabilities struct {
	Backends     []Backend `json:"backends"`
	Codecs       []Codec   `json:"codecs"`
	Selected     string    `json:"selected"`
	Probed       bool      `json:"-"`
	verification *verification
}

func codecCapabilities(hardware Capabilities) []Codec {
	backends := make([]transcodepolicy.Backend, len(hardware.Backends))
	for index, backend := range hardware.Backends {
		encoders := make(map[string]string)
		for codec := range backend.encoders {
			if encoder := backend.EncoderFor(codec); encoder != "" {
				encoders[codec] = encoder
			}
		}
		backends[index] = transcodepolicy.Backend{Encoders: encoders, Usable: backend.Usable || backend.verification != nil && backend.currentlyUsable()}
	}
	shared := transcodepolicy.Capabilities(backends)
	result := make([]Codec, len(shared))
	for index, codec := range shared {
		result[index] = Codec(codec)
	}
	return result
}

// Resolve applies automatic backend selection.
func (capabilities Capabilities) Resolve(configured string) string {
	if configured != "auto" {
		return configured
	}
	if capabilities.verification != nil {
		for _, backend := range capabilities.Backends {
			if backend.ID != "none" && backend.EncoderFor("h264") != "" {
				return backend.ID
			}
		}
		return "none"
	}
	return capabilities.Selected
}

// Backend returns the named backend or a stable unknown-backend view.
func (capabilities Capabilities) Backend(id string) Backend {
	for _, backend := range capabilities.Backends {
		if backend.ID == id {
			if backend.verification != nil {
				backend.Usable = backend.currentlyUsable()
			}
			return backend
		}
	}
	return Backend{ID: id, Name: id}
}

// Supports reports whether a configured backend is valid for this system.
func (capabilities Capabilities) Supports(id string) bool {
	return id == "auto" || capabilities.Backend(id).Supported
}

// Codec returns the named codec or a stable unknown-codec view.
func (capabilities Capabilities) Codec(id string) Codec {
	for _, codec := range capabilities.CurrentCodecs() {
		if codec.ID == id {
			return codec
		}
	}
	return Codec{ID: id, Name: id}
}

// SupportsCodec reports whether the codec setting can currently be used.
func (capabilities Capabilities) SupportsCodec(id string) bool {
	if id == "auto" || (!capabilities.Probed && transcodepolicy.ValidCodec(id)) {
		return true
	}
	if !transcodepolicy.ValidCodec(id) {
		return false
	}
	for _, backend := range capabilities.Backends {
		if (backend.Usable || backend.verification != nil) && backend.EncoderFor(id) != "" {
			return true
		}
	}
	return false
}

// ResolveEncoder applies Player's automatic and fallback encoder policy.
func (capabilities Capabilities) ResolveEncoder(configured, codec string) (string, string) {
	codec = transcodepolicy.NormalizeCodec(codec)
	if !capabilities.Probed {
		accelerator := capabilities.Resolve(configured)
		return accelerator, transcodepolicy.DefaultEncoder(accelerator, codec)
	}
	if configured != "auto" {
		backend := capabilities.Backend(configured)
		if backend.hasEncoder(codec) {
			return configured, backend.EncoderFor(codec)
		}
		return "none", capabilities.Backend("none").EncoderFor(codec)
	}
	for _, backend := range capabilities.Backends {
		if backend.ID != "none" && backend.hasEncoder(codec) {
			return backend.ID, backend.EncoderFor(codec)
		}
	}
	return "none", capabilities.Backend("none").EncoderFor(codec)
}

// PreferredCodec selects Player's best mutually supported client/hardware codec.
func (capabilities Capabilities) PreferredCodec(configured, accelerator string, client []string) string {
	if configured != "auto" {
		return transcodepolicy.NormalizeCodec(configured)
	}
	if !capabilities.Probed || len(client) == 0 {
		return "h264"
	}
	for _, codec := range []string{"h264", "hevc", "av1", "vp9"} {
		backend, encoder := capabilities.ResolveEncoder(accelerator, codec)
		if slices.Contains(client, codec) && backend != "none" && encoder != "" {
			return codec
		}
	}
	if slices.Contains(client, "h264") && capabilities.Backend("none").EncoderFor("h264") != "" {
		return "h264"
	}
	return ""
}

// HLSCodecs is a legacy estimate. HLS master publication reads the actual
// initialization data and never uses this estimate as output evidence.
func HLSCodecs(videoCodec, audioCodec, mode, outputCodec string, hasAudio bool) string {
	video := map[string]string{"h264": "avc1.64002a", "hevc": "hvc1", "vp9": "vp09", "av1": "av01"}[videoCodec]
	if mode == "transcode" || video == "" {
		video = map[string]string{"h264": "avc1.64002a", "hevc": "hvc1", "vp9": "vp09", "av1": "av01"}[transcodepolicy.NormalizeCodec(outputCodec)]
	}
	audio := ""
	if hasAudio {
		audio = map[string]string{"aac": "mp4a.40.2", "mp3": "mp4a.6B", "ac3": "ac-3", "eac3": "ec-3", "opus": "opus", "flac": "fLaC"}[audioCodec]
		if mode != "remux" || audio == "" {
			audio = "mp4a.40.2"
		}
	}
	if audio != "" {
		video += "," + audio
	}
	return video
}

func (capabilities Capabilities) CurrentCodecs() []Codec {
	if capabilities.verification == nil {
		return capabilities.Codecs
	}
	return codecCapabilities(capabilities)
}

func (backend Backend) hasEncoder(codec string) bool {
	return (backend.Usable || backend.verification != nil) && backend.EncoderFor(codec) != ""
}
