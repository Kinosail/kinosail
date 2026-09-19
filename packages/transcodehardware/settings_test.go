package transcodehardware

import (
	"reflect"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail/packages/transcodepolicy"
)

func TestSelectionNormalizationAndAcceleratorCatalog(t *testing.T) {
	if NormalizeAccelerator("") != "auto" || NormalizeAccelerator("none") != "none" {
		t.Fatal("accelerator normalization changed")
	}
	if NormalizeCodec("") != "auto" || NormalizeCodec("hevc") != "hevc" {
		t.Fatal("codec normalization changed")
	}
	for _, accelerator := range []string{"", "none", "auto", "vaapi", "qsv", "cuda", "videotoolbox", "rkmpp", "v4l2m2m", "amf", "mf"} {
		if !ValidAccelerator(accelerator) {
			t.Errorf("valid accelerator rejected: %q", accelerator)
		}
	}
	if ValidAccelerator("magic") || ValidAccelerator(strings.Repeat("x", 16<<10)) {
		t.Fatal("invalid accelerator accepted")
	}
}

func TestValidateSelectionPreservesPlayerPolicyAndNoInputMutation(t *testing.T) { //nolint:cyclop // One table verifies each ordered validation result.
	capabilities := settingsCapabilities()
	input := Selection{Transcoder: "automatic", ToneMap: true}
	selection, err := capabilities.ValidateSelection(input)
	if err != nil || selection != (Selection{Transcoder: "automatic", Codec: "auto", Accelerator: "auto", ToneMap: true}) || input.Codec != "" || input.Accelerator != "" {
		t.Fatalf("validated selection = %#v, input = %#v, error = %v", selection, input, err)
	}
	for _, test := range []struct {
		name      string
		selection Selection
		message   string
	}{
		{"missing quality", Selection{}, "transcoder quality is invalid"},
		{"oversized quality", Selection{Transcoder: strings.Repeat("x", 16<<10)}, "transcoder quality is invalid"},
		{"invalid codec", Selection{Transcoder: "speed", Codec: "vvc"}, "video codec is invalid"},
		{"oversized codec", Selection{Transcoder: "speed", Codec: strings.Repeat("x", 16<<10)}, "video codec is invalid"},
		{"unavailable codec", Selection{Transcoder: "quality", Codec: "hevc"}, "video codec is not available on this Server"},
		{"invalid accelerator", Selection{Transcoder: "automatic", Accelerator: "magic"}, "hardware accelerator is invalid"},
		{"oversized accelerator", Selection{Transcoder: "automatic", Accelerator: strings.Repeat("x", 16<<10)}, "hardware accelerator is invalid"},
		{"unsupported accelerator", Selection{Transcoder: "automatic", Accelerator: "vaapi"}, "hardware accelerator is not supported on this Server"},
	} {
		result, err := capabilities.ValidateSelection(test.selection)
		if err == nil || err.Error() != test.message || result != (Selection{}) {
			t.Errorf("%s = %#v, %v", test.name, result, err)
		}
	}
}

func TestReconcileSelectionPreservesExplicitChoicesAndRepairsDefaults(t *testing.T) { //nolint:cyclop // One table protects every persisted selection repair rule.
	capabilities := settingsCapabilities()
	supported := Selection{Transcoder: "speed", Accelerator: "", Codec: ""}
	result, changed, err := capabilities.ReconcileSelection(supported, SelectionSources{Accelerator: true, Codec: true})
	if err != nil || changed || result != supported {
		t.Fatalf("supported selection = %#v, changed %t, error %v", result, changed, err)
	}
	unsupported := Selection{Transcoder: "quality", Accelerator: "vaapi", Codec: "hevc", ToneMap: true}
	if result, changed, err = capabilities.ReconcileSelection(unsupported, SelectionSources{Accelerator: true}); err == nil || err.Error() != `transcoding.accelerator "vaapi" is not supported on this Server` || changed || result != (Selection{}) {
		t.Fatalf("explicit accelerator = %#v, changed %t, error %v", result, changed, err)
	}
	if result, changed, err = capabilities.ReconcileSelection(Selection{Accelerator: "auto", Codec: "hevc"}, SelectionSources{Codec: true}); err == nil || err.Error() != `transcoding.codec "hevc" is not available on this Server` || changed || result != (Selection{}) {
		t.Fatalf("explicit codec = %#v, changed %t, error %v", result, changed, err)
	}
	result, changed, err = capabilities.ReconcileSelection(unsupported, SelectionSources{})
	want := Selection{Transcoder: "quality", Accelerator: "auto", Codec: "auto", ToneMap: true}
	if err != nil || !changed || result != want {
		t.Fatalf("repaired selection = %#v, changed %t, error %v", result, changed, err)
	}
}

func TestSettingsResolvesPlayerProfilesAndErrors(t *testing.T) { //nolint:cyclop // One table verifies each stable conversion profile.
	unprobed := Capabilities{Selected: "none"}
	settings, err := unprobed.Settings(Selection{ToneMap: true}, "")
	want := transcodepolicy.Settings{Name: "automatic", Preset: "veryfast", CRF: "22", Codec: "h264", Accelerator: "none", Encoder: "libx264", Cache: "automatic:h264:auto:policy=3:hdr", ToneMap: true}
	if err != nil || !reflect.DeepEqual(settings, want) {
		t.Fatalf("automatic settings = %#v, error %v", settings, err)
	}
	capabilities := settingsCapabilities()
	for _, test := range []struct {
		selection     Selection
		codec         string
		preset, crf   string
		resolvedCodec string
	}{
		{Selection{Transcoder: "speed", Codec: "auto", Accelerator: "qsv"}, "", "ultrafast", "26", "h264"},
		{Selection{Transcoder: "quality", Codec: "auto", Accelerator: "qsv"}, "h264", "medium", "19", "h264"},
	} {
		settings, err = capabilities.Settings(test.selection, test.codec)
		if err != nil || settings.Preset != test.preset || settings.CRF != test.crf || settings.Codec != test.resolvedCodec || settings.Accelerator != "qsv" || settings.Encoder != "h264_qsv" || settings.Device != "/dev/dri/renderD129" {
			t.Errorf("settings = %#v, error %v", settings, err)
		}
	}
	for _, test := range []struct {
		name         string
		capabilities Capabilities
		codec        string
		message      string
	}{
		{"invalid codec", capabilities, "vvc", "video codec is not available on this Server"},
		{"unavailable codec", capabilities, "hevc", "video codec is not available on this Server"},
		{"missing encoder", Capabilities{Probed: true, Backends: []Backend{{ID: "none", Supported: true, Usable: true}}, Codecs: []Codec{{ID: "h264", Supported: true, Usable: true}}}, "h264", "video codec is not available on this Server"},
	} {
		settings, err = test.capabilities.Settings(Selection{Accelerator: "none"}, test.codec)
		if err == nil || err.Error() != test.message || settings != (transcodepolicy.Settings{}) {
			t.Errorf("%s = %#v, %v", test.name, settings, err)
		}
	}
}

func settingsCapabilities() Capabilities {
	return Capabilities{
		Selected: "qsv", Probed: true,
		Backends: []Backend{
			{ID: "none", Name: "Software", Supported: true, Usable: true, encoders: map[string]string{"h264": "libx264"}},
			{ID: "qsv", Name: "Intel", Device: "/dev/dri/renderD129", Supported: true, Usable: true, encoders: map[string]string{"h264": "h264_qsv"}},
			{ID: "vaapi", Name: "VA-API", Supported: false, encoders: map[string]string{"h264": "h264_vaapi"}},
		},
		Codecs: []Codec{{ID: "h264", Name: "AVC", Supported: true, Usable: true}, {ID: "hevc", Name: "HEVC", Supported: true}},
	}
}
