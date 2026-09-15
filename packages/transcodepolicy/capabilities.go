package transcodepolicy

// Capability describes the current support state for one Player codec.
type Capability struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Detected  bool   `json:"detected"`
	Supported bool   `json:"supported"`
	Usable    bool   `json:"usable"`
	Status    string `json:"status"`
	Reason    string `json:"reason,omitempty"`
	Action    string `json:"action,omitempty"`
}

// Backend supplies the detection facts needed by Player's codec policy.
type Backend struct {
	Encoders map[string]string
	Usable   bool
}

// Capabilities applies Player's codec readiness policy to detected backends.
func Capabilities(backends []Backend) []Capability { //nolint:cyclop // One explicit mapping keeps every codec state consistent.
	result := make([]Capability, 0, len(codecs))
	for _, definition := range codecs {
		detected, usable := false, false
		for _, backend := range backends {
			if backend.Encoders[definition.ID] != "" {
				detected, usable = true, usable || backend.Usable
			}
		}
		status, reason, action := "Ready", "Kinosail found a usable encoder.", ""
		switch {
		case definition.ID == "av2":
			status = "Not available yet"
			reason = "AV2 is standardized, but current FFmpeg builds and playback clients do not provide production encoding support"
			action = "Use H.264, HEVC, AV1, or VP9."
		case definition.ID == "vvc":
			status = "Not ready for streaming"
			reason = "Browser playback is not reliable enough for Kinosail streaming"
			if !detected {
				reason = "libvvenc is not present in the installed FFmpeg build; " + reason
			}
			action = "Use H.264, HEVC, AV1, or VP9."
		case !detected:
			status = "FFmpeg update needed"
			reason = "Not present in the installed FFmpeg build"
			action = "Install an FFmpeg build with a supported " + definition.Name + " encoder."
		case !usable:
			status = "Hardware setup needed"
			reason = "An encoder is present, but no usable device or software backend is available"
			action = "Review the hardware entries below, then run the local transcoder test."
		}
		result = append(result, Capability{ID: definition.ID, Name: definition.Name, Detected: detected, Supported: definition.Supported, Usable: definition.Supported && usable, Status: status, Reason: reason, Action: action})
	}
	return result
}
