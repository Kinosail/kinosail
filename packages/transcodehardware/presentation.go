package transcodehardware

import "strings"

const legacyHardwareOptionsHTML = `{{range .Hardware}}{{if ne .ID "none"}}<option value="{{.ID}}" {{if eq $.Accelerator .ID}}selected{{end}} {{if not .Supported}}disabled{{end}}>{{if eq .ID "vaapi"}}VA-API (AMD/Intel){{else}}{{.Name}}{{end}}</option>{{end}}{{end}}`

const hardwareOptionsHTML = `{{range .Hardware}}{{if and (ne .ID "none") .Relevant .Encoder}}<option value="{{.ID}}" {{if eq $.Accelerator .ID}}selected{{end}}>{{.FriendlyName}}</option>{{end}}{{end}}`

const legacyTranscoderSupportHTML = `<details><summary>Codec and hardware support</summary><h3>Video codecs</h3>{{range .Codecs}}<p><strong>{{.Name}}</strong>: {{if .Usable}}ready{{else if .Detected}}detected, unavailable{{else}}not detected{{end}}{{if .Reason}} · {{.Reason}}{{end}}</p>{{end}}<h3>Hardware</h3>{{range .Hardware}}<p><strong>{{.Name}}</strong>: {{if .Usable}}ready{{else if not .Supported}}not supported on this Server{{else if .Detected}}detected, unavailable{{else}}not detected{{end}}{{if .Reason}} · {{.Reason}}{{end}}</p>{{end}}</details>`

const transcoderSupportHTML = `<details class="capability-report"><summary>Video compatibility</summary><p>Automatic prefers a verified H.264 pipeline for broad compatibility. Other formats require matching device and encoder evidence.</p><h3>Video formats</h3><div class="capability-list">{{range .Codecs}}<article><header><strong>{{.Name}}</strong><span>{{.SimpleStatus}}</span></header><p>{{.SimpleReason}}</p>{{if .Action}}<details class="capability-technical"><summary>Technical details</summary><p>{{.Reason}}</p><p><strong>Next step:</strong> {{.Action}}</p></details>{{end}}</article>{{end}}</div><h3>Ways to convert video</h3><div class="capability-list">{{range .Hardware}}{{if .Relevant}}<article><header><strong>{{.FriendlyName}}</strong><span>{{.SimpleStatus}}</span></header><p>{{.SimpleReason}}</p><details class="capability-technical"><summary>Technical details</summary><p><strong>FFmpeg option:</strong> {{.Name}}</p><p>{{.Reason}}</p>{{if .Action}}<p><strong>Next step:</strong> {{.Action}}</p>{{end}}</details></article>{{end}}{{end}}</div></details>`

var transcoderCopy = strings.NewReplacer(
	legacyHardwareOptionsHTML, hardwareOptionsHTML,
	legacyTranscoderSupportHTML, transcoderSupportHTML,
	`<h2>Transcoder</h2><p>Automatic uses H.264 for broad device support. Choose a newer codec only when every playback device supports it.</p>`, `<h2>Video conversion</h2><p>Automatic checks each playback device only when Kinosail must convert video. It prefers H.264 and uses only encoding paths that passed a local HLS smoke check. A manual choice forces that format for every playback device.</p>`,
	`<label>Quality <select name="quality">`, `<label>Conversion preference <select name="quality">`,
	`<label>Video codec <select name="codec">`, `<label>Video format <select name="codec">`,
	`>Automatic · H.264 ({{t "Recommended"}})</option>`, `>Automatic per device ({{t "Recommended"}})</option>`,
	`<label>Hardware <select name="accelerator"><option value="none" {{if eq .Accelerator "none"}}selected{{end}}>Software</option>`, `<label>Speed up with <select name="accelerator"><option value="none" {{if eq .Accelerator "none"}}selected{{end}}>Processor</option>`,
	`</select></label><label><input type="checkbox" name="toneMap"`, `</select></label><p class="automatic-hardware"><strong>Automatic will use:</strong> {{.AutomaticHardware}}</p><label><input type="checkbox" name="toneMap"`,
	`> Tone-map HDR to SDR</label>`, `> Improve HDR colors on non-HDR screens</label>`,
	`Local transcoder test`, `Quick compatibility check`,
	`Transcoding is ready`, `Video conversion is ready`,
	`Kinosail encoded a local test clip with`, `Kinosail converted a small test video with`,
	`This confirms FFmpeg, scaling, {{.TranscoderTest.CodecName}}, AAC, and the selected encoder. No Library Content is used. Actual HDR, subtitles, networks, and playback devices can still vary.`, `The basic video tools work. Your files and playback devices can still behave differently.`,
	`Transcoder test failed`, `Video check failed`,
	`Choose Automatic or Software, save, and test again. If hardware is selected, make sure its GPU device is available to the Server container.`, `Choose Automatic or Processor, save, and check again. Open Video compatibility if hardware needs setup.`,
	`Is local transcoding ready?`, `Can this Server convert video?`,
	`Create a tiny synthetic clip on this Server to check FFmpeg, scaling, {{.TranscoderTest.CodecName}}, AAC, and <strong>{{.TranscoderTest.Backend}}</strong>. No Library Content is used.`, `Kinosail makes a small test video. It does not use anything from your library.`,
	`Run local test`, `Run quick check`,
	`Test again`, `Check again`,
)

// EnhanceSettingsPage applies Player's hardware presentation to a settings template.
func EnhanceSettingsPage(page string) string {
	return transcoderCopy.Replace(page)
}

// FriendlyName returns Player's plain-language backend name.
func (backend Backend) FriendlyName() string {
	if name := map[string]string{"none": "Processor", "qsv": "Intel graphics", "cuda": "NVIDIA graphics", "vaapi": "AMD or Intel graphics", "rkmpp": "Rockchip graphics", "v4l2m2m": "Built-in Linux video hardware", "videotoolbox": "Apple graphics", "amf": "AMD graphics", "mf": "Windows video hardware", "asahi": "Apple Silicon"}[backend.ID]; name != "" {
		return name
	}
	return backend.Name
}

// SimpleStatus returns the status shown in Player settings.
func (backend Backend) SimpleStatus() string {
	if backend.currentlyUsable() {
		if backend.ID == "none" {
			return "Smoke check passed"
		}
		return "Smoke check passed"
	}
	if backend.Detected {
		return "Needs setup"
	}
	switch backend.ID { //nolint:gocritic // Explicit value dispatch keeps mutation coverage deterministic.
	case "asahi":
		return "Processor only"
	}
	if backend.Supported {
		return "Not available"
	}
	return "Other system"
}

// SimpleReason returns the explanation shown in Player settings.
func (backend Backend) SimpleReason() string {
	if backend.ID == "none" {
		if backend.currentlyUsable() {
			return "The local HLS smoke check passed. Processor speed and real media still affect playback."
		}
		return "Kinosail needs working video tools before it can convert video."
	}
	switch backend.ID { //nolint:gocritic // Explicit value dispatch keeps mutation coverage deterministic.
	case "asahi":
		return "Apple hardware video encoding is not available on Asahi Linux yet. Automatic uses the processor."
	}
	switch {
	case backend.currentlyUsable():
		return "A local HLS smoke check passed. This does not certify every input format or playback device."
	case backend.Detected:
		return "Kinosail found this option, but no encoding operation is currently verified."
	case backend.Supported:
		return "This option is not included with the installed video tools. Automatic skips it."
	default:
		return "This option is for a different system."
	}
}

// SimpleStatus returns the status shown in Player settings.
func (codec Codec) SimpleStatus() string {
	if codec.Usable {
		if codec.ID == "h264" {
			return "Recommended"
		}
		return "Available"
	}
	if !codec.Supported {
		return "Not ready yet"
	}
	return "Not available"
}

// SimpleReason returns the explanation shown in Player settings.
func (codec Codec) SimpleReason() string {
	if !codec.Usable {
		if !codec.Supported {
			return "Kinosail does not use this format yet because playback support is limited."
		}
		return "The installed video tools cannot create this format on this Server."
	}
	return map[string]string{"h264": "Works with the widest range of TVs, phones, and browsers.", "hevc": "Can save bandwidth on newer playback devices.", "av1": "Can save more bandwidth, but some older devices cannot play it.", "vp9": "Useful for compatible browsers and streaming devices."}[codec.ID]
}

func (backend Backend) currentlyUsable() bool {
	if backend.verification == nil {
		return false
	}
	for codec := range backend.encoders {
		if backend.EncoderFor(codec) != "" {
			return true
		}
	}
	return false
}
