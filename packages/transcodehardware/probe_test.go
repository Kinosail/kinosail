package transcodehardware

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
)

func TestHardwareDefinitionsMatchPlayerPlatforms(t *testing.T) {
	t.Parallel()
	tests := []struct {
		os, arch, platform string
		want               map[string]bool
	}{
		{"linux", "amd64", "", map[string]bool{"qsv": true, "cuda": true, "vaapi": true, "v4l2m2m": true}},
		{"linux", "arm64", "", map[string]bool{"cuda": true, "vaapi": true, "rkmpp": true, "v4l2m2m": true}},
		{"darwin", "arm64", "", map[string]bool{"videotoolbox": true}},
		{"windows", "amd64", "", map[string]bool{"qsv": true, "cuda": true, "amf": true, "mf": true}},
		{"windows", "arm64", "", map[string]bool{"mf": true}},
		{"freebsd", "amd64", "", map[string]bool{"vaapi": true}},
		{"other", "other", "asahi", map[string]bool{}},
	}
	for _, test := range tests {
		for _, definition := range hardwareDefinitions(test.os, test.arch, func(...string) bool { return true }, test.platform) {
			if definition.supported != test.want[definition.id] {
				t.Errorf("%s/%s %s supported = %v, want %v", test.os, test.arch, definition.id, definition.supported, test.want[definition.id])
			}
			if definition.id == "asahi" && definition.relevant != (test.platform == "asahi") {
				t.Errorf("%s/%s Asahi relevance = %v", test.os, test.arch, definition.relevant)
			}
		}
	}
}

func TestProbePlatformDefaultsEachMissingField(t *testing.T) {
	t.Parallel()
	if got := probePlatform(ProbeOptions{GOOS: "linux"}); got.GOOS != "linux" || got.GOARCH == "" {
		t.Fatalf("architecture default = %#v", got)
	}
	if got := probePlatform(ProbeOptions{GOARCH: "arm64"}); got.GOOS == "" || got.GOARCH != "arm64" {
		t.Fatalf("operating-system default = %#v", got)
	}
}

func TestProbeDiscoversPortableBackendsAndCodecs(t *testing.T) { //nolint:cyclop // One probe must expose coherent backend and codec evidence.
	t.Parallel()
	ffmpeg := fakeFFmpeg(t, "libx264 libx265 libsvtav1 libvpx-vp9 libvvenc h264_qsv hevc_qsv av1_qsv vp9_qsv qsv h264_v4l2m2m hevc_v4l2m2m h264_mf hevc_mf av1_mf")
	video := filepath.Join(t.TempDir(), "video11")
	linux := probe(context.Background(), ProbeOptions{Application: "Kinosail Player", FFmpeg: ffmpeg, Devices: []string{video}, Enabled: true, GOOS: "linux", GOARCH: "arm64"}, func(string) bool { return true }, func() string { return "" })
	if backend := linux.Backend("v4l2m2m"); !backend.Usable || backend.Device != video || backend.Status != "Ready to test" {
		t.Fatalf("V4L2 backend = %#v", backend)
	}
	windows := probe(context.Background(), ProbeOptions{Application: "Kinosail Player", FFmpeg: ffmpeg, Enabled: true, GOOS: "windows", GOARCH: "arm64"}, func(string) bool { return false }, func() string { return "" })
	if backend := windows.Backend("mf"); !backend.Usable || backend.Status != "Ready to test" || len(backend.Codecs) != 3 {
		t.Fatalf("Media Foundation backend = %#v", backend)
	}
	for _, id := range []string{"h264", "hevc", "av1", "vp9"} {
		if codec := windows.Codec(id); !codec.Detected || !codec.Supported || !codec.Usable {
			t.Errorf("%s capability = %#v", id, codec)
		}
	}
	if codec := windows.Codec("vvc"); !codec.Detected || codec.Supported || codec.Usable || codec.Reason == "" {
		t.Errorf("VVC capability = %#v", codec)
	}
}

func TestPortableBackendDeviceRequirements(t *testing.T) {
	t.Parallel()
	never := func(...string) bool { return false }
	nvidia := func(names ...string) bool { return slices.Contains(names, "nvidia") }
	device := func(definitions []hardwareDefinition, id string) bool {
		t.Helper()
		for _, definition := range definitions {
			if definition.id == id {
				return definition.device
			}
		}
		t.Fatalf("missing %s definition", id)
		return false
	}
	if !device(hardwareDefinitions("windows", "amd64", never, ""), "cuda") {
		t.Fatal("Windows CUDA should not require a Linux device node")
	}
	if device(hardwareDefinitions("linux", "amd64", never, ""), "cuda") {
		t.Fatal("Linux CUDA accepted a missing NVIDIA device")
	}
	if !device(hardwareDefinitions("linux", "amd64", nvidia, ""), "cuda") {
		t.Fatal("Linux CUDA rejected an available NVIDIA device")
	}
}

func TestProbeRequiresCompleteAccessibleHardware(t *testing.T) {
	t.Parallel()
	ffmpeg := fakeFFmpeg(t, "libx264 h264_qsv qsv h264_rkmpp rkmpp")
	render := "/dev/dri/renderD128"
	options := ProbeOptions{Application: "Kinosail Player", FFmpeg: ffmpeg, Devices: []string{render}, Enabled: true, GOOS: "linux", GOARCH: "amd64"}
	denied := probe(context.Background(), options, func(string) bool { return false }, func() string { return "" })
	if denied.Selected != "none" || denied.Backend("qsv").Usable || denied.Backend("qsv").Reason == "" {
		t.Fatalf("denied render device = %#v", denied.Backend("qsv"))
	}
	rkmppDevices := []string{render, "/dev/dma_heap/system", "/dev/mpp_service", "/dev/rga"}
	options.Devices, options.GOARCH = rkmppDevices, "arm64"
	partial := probe(context.Background(), options, func(path string) bool { return path != "/dev/rga" }, func() string { return "" })
	if partial.Backend("rkmpp").Usable {
		t.Fatal("RKMPP accepted an incomplete device set")
	}
	complete := probe(context.Background(), options, func(string) bool { return true }, func() string { return "asahi" })
	if complete.Selected != "rkmpp" || !complete.Backend("rkmpp").Usable || !complete.Backend("asahi").Relevant {
		t.Fatalf("complete RKMPP = %#v", complete)
	}
}

func TestProbeSoftwareStatesAndDefaults(t *testing.T) {
	t.Parallel()
	states := []struct {
		options ProbeOptions
		status  string
	}{
		{ProbeOptions{Application: "Kinosail Player"}, "Detection off"},
		{ProbeOptions{Application: "Kinosail Player", Enabled: true}, "FFmpeg unavailable"},
		{ProbeOptions{Application: "Kinosail Player", Enabled: true, FFmpeg: filepath.Join(t.TempDir(), "missing")}, "FFmpeg unavailable"},
		{ProbeOptions{Application: "Kinosail Player", Enabled: true, FFmpeg: fakeFFmpeg(t, "libx264rgb")}, "FFmpeg update needed"},
		{ProbeOptions{Application: "Kinosail Player", Enabled: true, FFmpeg: fakeFFmpeg(t, "libx264")}, "Ready"},
	}
	for _, test := range states {
		capabilities := probe(context.Background(), test.options, func(string) bool { return false }, func() string { return "" })
		if got := capabilities.Backend("none").Status; got != test.status {
			t.Errorf("software status = %q, want %q", got, test.status)
		}
	}
	if got := Probe(context.Background(), ProbeOptions{Application: "Kinosail Player"}); got.Probed || got.Selected != "none" {
		t.Fatalf("default probe = %#v", got)
	}
}

func TestHardwareMessagesCoverEveryState(t *testing.T) {
	t.Parallel()
	definition := hardwareDefinition{id: "qsv", name: "Intel", encoder: "h264_qsv", supported: true, reason: "reason", action: "action"}
	tests := []struct {
		definition               hardwareDefinition
		probed, detected, usable bool
		status                   string
	}{
		{hardwareDefinition{id: "asahi", reason: "reason"}, true, false, false, "Software encoding only"},
		{hardwareDefinition{reason: "reason"}, true, false, false, "Different system"},
		{definition, false, false, false, "Detection off"},
		{definition, true, false, false, "FFmpeg update needed"},
		{definition, true, true, false, "Device access needed"},
		{definition, true, true, true, "Ready to test"},
	}
	for _, test := range tests {
		status, reason, action := hardwareMessage(test.definition, "Kinosail Player", test.probed, test.detected, test.usable)
		if status != test.status || reason == "" || action == "" {
			t.Errorf("message = %q, %q, %q", status, reason, action)
		}
	}
}

func TestEmptyHardwareRequirementIsSatisfied(t *testing.T) {
	t.Parallel()
	if !hasDevices(func(...string) bool { return false }) {
		t.Fatal("an empty hardware requirement was rejected")
	}
}

func TestDeviceDiscoveryAndBoundedProbeOutput(t *testing.T) { //nolint:cyclop // One fixture covers directory discovery and its bounded output primitive.
	t.Parallel()
	directory := t.TempDir()
	paths := []string{"renderD128", "nvidia0", "mpp_service", "rga", "video11", "video", "unknown"}
	for _, name := range paths {
		if err := os.WriteFile(filepath.Join(directory, name), nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	dma := filepath.Join(directory, "dma_heap", "system")
	if err := os.MkdirAll(filepath.Dir(dma), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dma, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	kinds := accessibleHardwareDeviceKinds([]string{directory, dma, filepath.Join(directory, "missing")}, func(string) bool { return true })
	if !reflect.DeepEqual(keys(kinds), []string{"dma_heap", "mpp", "nvidia", "render", "rga", "video"}) {
		t.Fatalf("device kinds = %#v", kinds)
	}
	if !canOpenHardwareDevice(filepath.Join(directory, "renderD128")) || canOpenHardwareDevice(filepath.Join(directory, "missing")) {
		t.Fatal("device accessibility did not fail closed")
	}
	buffer := cappedBuffer{remaining: 3}
	if written, err := buffer.Write([]byte("hello")); err != nil || written != 5 || buffer.String() != "hel" {
		t.Fatalf("bounded write = %d, %v, %q", written, err, buffer.String())
	}
	if written, err := buffer.Write([]byte("x")); err != nil || written != 1 || buffer.String() != "hel" {
		t.Fatalf("exhausted write = %d, %v, %q", written, err, buffer.String())
	}
}

func TestLinuxHardwarePlatformChecksBothLocations(t *testing.T) {
	t.Parallel()
	_ = linuxHardwarePlatform()
	read := func(path string) ([]byte, error) {
		if strings.Contains(path, "/sys/") {
			return []byte("Apple,j314s"), nil
		}
		return nil, errors.New("missing")
	}
	if linuxHardwarePlatformWith(read) != "asahi" {
		t.Fatal("Apple device tree was not recognized")
	}
	if linuxHardwarePlatformWith(func(string) ([]byte, error) { return []byte("rockchip"), nil }) != "" {
		t.Fatal("non-Apple platform was recognized")
	}
}

func TestDeviceKindsRejectLookalikes(t *testing.T) {
	t.Parallel()
	tests := map[string]string{
		"/dev/dri/renderD128":  "render",
		"/dev/nvidia0":         "nvidia",
		"/dev/mpp_service":     "mpp",
		"/dev/rga":             "rga",
		"/dev/dma_heap/system": "dma_heap",
		"/dev/video11":         "video",
		"/dev/video":           "",
		"/dev/video-card":      "",
		"/dev/other":           "",
	}
	for path, want := range tests {
		if got := hardwareDeviceKind(path); got != want {
			t.Errorf("hardwareDeviceKind(%q) = %q, want %q", path, got, want)
		}
	}
}

func fakeFFmpeg(t *testing.T, capabilities string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "ffmpeg")
	script := "#!/bin/sh\nprintf '%s\\n' '" + capabilities + "'\nprintf 'stderr-capability\\n' >&2\n"
	if err := os.WriteFile(path, []byte(script), 0o700); err != nil { //nolint:gosec // Test fixture must be executable.
		t.Fatal(err)
	}
	return path
}

func keys(values map[string]string) []string {
	result := make([]string, 0, len(values))
	for key := range values {
		result = append(result, key)
	}
	slices.Sort(result)
	return result
}
