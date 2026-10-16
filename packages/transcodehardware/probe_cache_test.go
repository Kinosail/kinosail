package transcodehardware

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail/packages/transcodepolicy"
)

func TestProvisionalCapabilitiesUseSoftwareUntilVerified(t *testing.T) {
	provisional := Provisional(ProbeOptions{Application: "Kinosail Player", FFmpeg: "ffmpeg", Enabled: true, GOOS: "linux", GOARCH: "amd64"})
	encoded, err := json.Marshal(provisional)
	if err != nil || !provisional.Probing || !strings.Contains(string(encoded), `"probing":true`) {
		t.Fatalf("provisional state = %s, %v", encoded, err)
	}
	for _, selection := range []Selection{{Transcoder: "automatic", Accelerator: "vaapi"}, {Transcoder: "automatic", Accelerator: "auto", Codec: "hevc"}} {
		settings, err := provisional.Settings(selection, "")
		if err != nil || settings.Accelerator != "none" || settings.Encoder != transcodepolicy.DefaultEncoder("none", settings.Codec) || settings.HardwareDecode || settings.Device != "" {
			t.Fatalf("provisional settings for %+v = %#v, %v", selection, settings, err)
		}
	}
	if backend := provisional.Backend("vaapi"); backend.Usable || !backend.Supported || backend.Status != "Checking hardware" {
		t.Fatalf("provisional backend = %#v", backend)
	}
	kept, changed, err := provisional.ReconcileSelection(Selection{Transcoder: "automatic", Accelerator: "vaapi", Codec: "av1"}, SelectionSources{Accelerator: true, Codec: true})
	if err != nil || changed || kept.Accelerator != "vaapi" || kept.Codec != "av1" {
		t.Fatalf("provisional reconciliation changed the saved choice: %#v, %t, %v", kept, changed, err)
	}
	if _, _, err := provisional.ReconcileSelection(Selection{Accelerator: "videotoolbox"}, SelectionSources{Accelerator: true}); err == nil {
		t.Fatal("provisional reconciliation accepted an accelerator for another platform")
	}
}

func cachedFixture(t *testing.T) (ProbeOptions, Capabilities, string) {
	t.Helper()
	tools, devices := t.TempDir(), t.TempDir()
	ffmpeg := fakeFFmpeg(t, "libx264 libx265 h264_vaapi hevc_vaapi vaapi")
	if err := os.WriteFile(filepath.Join(devices, "renderD128"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	options := ProbeOptions{Application: "Kinosail Player", FFmpeg: ffmpeg, Devices: []string{devices}, Enabled: true, GOOS: "linux", GOARCH: "amd64"}
	verified := verify(t.Context(), options, probe(t.Context(), options, func(string) bool { return true }, func() string { return "" }), func(_ context.Context, _ string, settings transcodepolicy.Settings, _ string) transcodepolicy.CheckResult {
		status := "passed"
		if settings.Codec == "hevc" && settings.Accelerator == "vaapi" || settings.ToneMap {
			status = "failed"
		}
		return transcodepolicy.CheckResult{Status: status, HardwareDecode: settings.HardwareDecode}
	})
	return options, verified, filepath.Join(tools, "hardware-probe.json")
}

func TestCachedProbeRestoresVerifiedEvidence(t *testing.T) {
	options, verified, path := cachedFixture(t)
	if _, ok := CachedProbe(path, options); ok {
		t.Fatal("missing cache was accepted")
	}
	if err := StoreProbe(path, options, verified); err != nil {
		t.Fatal(err)
	}
	restored, ok := CachedProbe(path, options)
	if !ok || restored.Probing || !restored.Probed || restored.Selected != verified.Selected || restored.Selected != "vaapi" {
		t.Fatalf("restored = %#v, %t", restored, ok)
	}
	for _, codec := range []string{"h264", "hevc"} {
		want, wantErr := verified.Settings(Selection{Transcoder: "automatic", Accelerator: "auto"}, codec)
		got, gotErr := restored.Settings(Selection{Transcoder: "automatic", Accelerator: "auto"}, codec)
		if got != want || (gotErr == nil) != (wantErr == nil) {
			t.Fatalf("%s settings = %#v, %v; want %#v, %v", codec, got, gotErr, want, wantErr)
		}
	}
	if !restored.SupportsCodec("hevc") || restored.Backend("vaapi").EncoderFor("hevc") != "" || restored.Backend("vaapi").EncoderFor("h264") != "h264_vaapi" {
		t.Fatalf("restored evidence = %#v", restored.Backend("vaapi"))
	}
	restored.RecordFailure(transcodepolicy.Settings{Accelerator: "vaapi", Codec: "h264", Device: restored.Backend("vaapi").Device})
	if restored.Backend("vaapi").EncoderFor("h264") != "" {
		t.Fatal("restored evidence did not accept runtime failure evidence")
	}
}

func TestCachedProbeRejectsChangedInstallationsAndTampering(t *testing.T) {
	options, verified, path := cachedFixture(t)
	store := func() {
		t.Helper()
		if err := StoreProbe(path, options, verified); err != nil {
			t.Fatal(err)
		}
	}
	store()
	added := filepath.Join(options.Devices[0], "renderD129")
	if err := os.WriteFile(added, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, ok := CachedProbe(path, options); ok {
		t.Fatal("cache survived a changed device set")
	}
	if err := os.Remove(added); err != nil {
		t.Fatal(err)
	}
	if _, ok := CachedProbe(path, options); !ok {
		t.Fatal("cache was not accepted for the original device set")
	}
	if err := os.WriteFile(options.FFmpeg, []byte("#!/bin/sh\nexit 0\n# replaced\n"), 0o700); err != nil { //nolint:gosec // Test fixture must be executable.
		t.Fatal(err)
	}
	if _, ok := CachedProbe(path, options); ok {
		t.Fatal("cache survived a replaced FFmpeg executable")
	}
	options, verified, path = cachedFixture(t)
	store()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	device := verified.Backend("vaapi").Device
	for name, tampered := range map[string]string{
		"device":   strings.ReplaceAll(string(data), `"`+device+`"`, `"/dev/../etc/passwd"`),
		"encoder":  strings.Replace(string(data), `"h264_vaapi"`, `"-filter_complex"`, 1),
		"backend":  strings.Replace(string(data), `"id":"vaapi"`, `"id":"unknown"`, 1),
		"garbage":  "{",
		"oversize": strings.Repeat(" ", 2<<20) + string(data),
	} {
		if err := os.WriteFile(path, []byte(tampered), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, ok := CachedProbe(path, options); ok {
			t.Errorf("tampered %s cache was accepted", name)
		}
	}
	disabled := options
	disabled.Enabled = false
	if err := StoreProbe(path, disabled, Probe(t.Context(), disabled)); err == nil {
		t.Fatal("unverified capabilities were cached")
	}
}
