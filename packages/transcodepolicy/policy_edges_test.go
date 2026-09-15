package transcodepolicy

import (
	"strings"
	"testing"
)

func TestVideoArgumentsCoverBackendAndSoftwareEdges(t *testing.T) {
	for _, test := range []struct {
		name     string
		settings Settings
		want     string
	}{
		{"unresolved automatic uses software", Settings{Accelerator: "auto", CRF: "22"}, "-c:v libx264"},
		{"VAAPI tone map", Settings{Accelerator: "vaapi", Device: "/dev/custom", ToneMap: true, CRF: "22"}, "tonemap=hable"},
		{"QSV device", Settings{Accelerator: "qsv", Device: "/dev/custom", CRF: "22"}, "-init_hw_device vaapi=kino_va:/dev/custom"},
		{"AOM", Settings{Encoder: "libaom-av1", CRF: "22"}, "-cpu-used 6"},
		{"Rav1e", Settings{Encoder: "librav1e", CRF: "22"}, "-speed 8"},
	} {
		t.Run(test.name, func(t *testing.T) {
			input, video := VideoArguments(test.settings, "1280")
			if arguments := strings.Join(append(input, video...), " "); !strings.Contains(arguments, test.want) {
				t.Fatalf("arguments %q lack %q", arguments, test.want)
			}
		})
	}
	if got := renderDevice("/dev/custom"); got != "/dev/custom" {
		t.Fatalf("render device = %q", got)
	}
}

func TestCodecSelectionCoversEmptyAndUnavailableChoices(t *testing.T) {
	if !ValidCodec("") || !ValidCodec("auto") {
		t.Fatal("automatic codecs were rejected")
	}
	if got := DefaultEncoder("none", "h264"); got != "libx264" {
		t.Fatalf("software encoder = %q", got)
	}
	if got := DefaultEncoder("none", "av2"); got != "" {
		t.Fatalf("unavailable software encoder = %q", got)
	}
	if got := FirstEncoder("libx264", []string{"libx265"}); got != "" {
		t.Fatalf("missing encoder = %q", got)
	}
}

func TestCapabilitiesExplainUndetectedVVC(t *testing.T) {
	for _, capability := range Capabilities(nil) {
		if capability.ID == "vvc" && !strings.Contains(capability.Reason, "libvvenc is not present") {
			t.Fatalf("VVC reason = %q", capability.Reason)
		}
	}
}
