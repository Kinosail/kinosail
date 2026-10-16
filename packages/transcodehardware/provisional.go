package transcodehardware

import "github.com/MikeO7/kinosail/packages/transcodepolicy"

const checkingReason = "Kinosail is checking FFmpeg and this device in the background. Playback uses software encoding until the check finishes."

func Provisional(options ProbeOptions) Capabilities {
	options = probePlatform(options)
	encoders := make(map[string]string)
	for _, definition := range transcodepolicy.Codecs() {
		if definition.Supported && len(definition.Software) > 0 {
			encoders[definition.ID] = definition.Software[0]
		}
	}
	capabilities := Capabilities{Selected: "none", Probing: true, Backends: []Backend{{
		ID: "none", Name: "Software (libx264)", Encoder: encoders["h264"], Codecs: transcodepolicy.EncoderCodecIDs(encoders), Supported: true, Detected: true, Usable: true, Relevant: true,
		Status: "Checking hardware", Reason: checkingReason, encoders: encoders,
	}}}
	for _, definition := range hardwareDefinitions(options.GOOS, options.GOARCH, func(...string) bool { return false }, "") {
		status, reason, action := hardwareMessage(definition, options.Application, false, false, false)
		if definition.supported {
			status, reason, action = "Checking hardware", checkingReason, ""
		}
		capabilities.Backends = append(capabilities.Backends, Backend{ID: definition.id, Name: definition.name, Encoder: definition.encoder, Supported: definition.supported, Relevant: definition.relevant || definition.supported, Status: status, Reason: reason, Action: action})
	}
	capabilities.Codecs = codecCapabilities(capabilities)
	return capabilities
}
