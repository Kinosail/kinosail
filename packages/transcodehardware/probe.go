// Package transcodehardware owns Player's FFmpeg hardware discovery and
// per-device codec selection policy for Kinosail media applications.
package transcodehardware

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/MikeO7/kinosail/packages/transcodepolicy"
)

// ProbeOptions supplies installation-specific hardware probe inputs.
type ProbeOptions struct {
	Application string
	FFmpeg      string
	Devices     []string
	Enabled     bool
	GOOS        string
	GOARCH      string
}

// Probe discovers FFmpeg encoders and accessible hardware devices.
func Probe(ctx context.Context, options ProbeOptions) Capabilities {
	return verify(ctx, options, probe(ctx, options, canOpenHardwareDevice, linuxHardwarePlatform), transcodepolicy.Check)
}

func probe(ctx context.Context, options ProbeOptions, canOpen func(string) bool, platform func() string) Capabilities {
	options = probePlatform(options)
	available, encoderError := ffmpegEvidence(ctx, options)
	capabilities := Capabilities{Backends: []Backend{softwareBackend(options, available, encoderError)}, Selected: "none", Probed: options.Enabled}
	deviceKinds := accessibleHardwareDeviceKinds(hardwareDevices(options.Devices), canOpen)
	hasDevice := func(names ...string) bool {
		for _, name := range names {
			if deviceKinds[name] != "" {
				return true
			}
		}
		return false
	}
	platformName := ""
	if options.GOOS == "linux" && options.GOARCH == "arm64" {
		platformName = platform()
	}
	for _, definition := range hardwareDefinitions(options.GOOS, options.GOARCH, hasDevice, platformName) {
		backend := detectedBackend(definition, options, available, deviceKinds)
		capabilities.Backends = append(capabilities.Backends, backend)
		if capabilities.Selected == "none" && backend.Usable {
			capabilities.Selected = backend.ID
		}
	}
	capabilities.Codecs = codecCapabilities(capabilities)
	return capabilities
}

func probePlatform(options ProbeOptions) ProbeOptions {
	if options.GOOS == "" {
		options.GOOS = runtime.GOOS
	}
	if options.GOARCH == "" {
		options.GOARCH = runtime.GOARCH
	}
	return options
}

func ffmpegEvidence(ctx context.Context, options ProbeOptions) (string, error) {
	available, encoderError := "", error(nil)
	if options.Enabled && options.FFmpeg != "" {
		probeCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		encoders, err := ffmpegCapabilities(probeCtx, options.FFmpeg, "-encoders")
		encoderError = err
		hwaccels, _ := ffmpegCapabilities(probeCtx, options.FFmpeg, "-hwaccels")
		available = strings.ToLower(encoders + "\n" + hwaccels)
	}
	return available, encoderError
}

func softwareBackend(options ProbeOptions, available string, encoderError error) Backend { //nolint:cyclop // Player's existing FFmpeg state contract stays explicit.
	softwareEncoders := make(map[string]string)
	for _, definition := range transcodepolicy.Codecs() {
		if encoder := transcodepolicy.FirstEncoder(available, definition.Software); encoder != "" {
			softwareEncoders[definition.ID] = encoder
		}
	}
	softwareDetected := options.Enabled && options.FFmpeg != "" && encoderError == nil && len(softwareEncoders) > 0
	status, reason, action := "Ready", "Software encoding works on every system, but it uses the CPU.", "Run the local transcoder test to confirm this FFmpeg installation."
	if !options.Enabled {
		status, reason, action = "Detection off", options.Application+" did not inspect FFmpeg.", "Enable detection or run the local transcoder test."
	} else {
		switch options.FFmpeg {
		case "":
			status, reason, action = ffmpegUnavailableMessage()
		default:
			switch encoderError {
			case nil:
				if !softwareDetected {
					status, reason, action = "FFmpeg update needed", "This FFmpeg build has no supported software video encoder.", "Install an FFmpeg build with libx264."
				}
			default:
				status, reason, action = ffmpegUnavailableMessage()
			}
		}
	}
	return Backend{ID: "none", Name: "Software (libx264)", Encoder: softwareEncoders["h264"], Codecs: transcodepolicy.EncoderCodecIDs(softwareEncoders), Supported: true, Detected: softwareDetected, Usable: softwareDetected, Relevant: true, Status: status, Reason: reason, Action: action, encoders: softwareEncoders}
}

func ffmpegUnavailableMessage() (string, string, string) {
	return "FFmpeg unavailable", "Kinosail could not run the configured FFmpeg executable.", "Install FFmpeg or correct the KINOSAIL_FFMPEG path."
}

func hardwareDevices(devices []string) []string {
	if len(devices) == 0 {
		return []string{"/dev", "/dev/dri", "/dev/dma_heap", "/dev/nvidia0", "/dev/mpp_service", "/dev/rga"}
	}
	return devices
}

func detectedBackend(definition hardwareDefinition, options ProbeOptions, available string, deviceKinds map[string]string) Backend { //nolint:cyclop // One backend is assembled from the coherent probe evidence.
	detected := definition.encoder != "" && transcodepolicy.HasCapability(available, definition.encoder) && (definition.hw == "" || transcodepolicy.HasCapability(available, definition.hw))
	usable := definition.supported && detected && definition.device
	status, reason, action := hardwareMessage(definition, options.Application, options.Enabled && options.FFmpeg != "", detected, usable)
	encoders := make(map[string]string)
	for _, codec := range transcodepolicy.Codecs() {
		if encoder := codec.Hardware[definition.id]; encoder != "" && transcodepolicy.HasCapability(available, encoder) {
			encoders[codec.ID] = encoder
		}
	}
	device := ""
	switch definition.id {
	case "qsv", "vaapi":
		device = deviceKinds["render"]
	case "v4l2m2m":
		device = deviceKinds["video"]
	}
	return Backend{ID: definition.id, Name: definition.name, Encoder: definition.encoder, Codecs: transcodepolicy.EncoderCodecIDs(encoders), Device: device, Supported: definition.supported, Detected: detected, Usable: usable, Relevant: definition.relevant || definition.supported || detected, Status: status, Reason: reason, Action: action, encoders: encoders}
}

func ffmpegCapabilities(ctx context.Context, ffmpeg, argument string) (string, error) {
	stdout, stderr := cappedBuffer{remaining: 256 << 10}, cappedBuffer{remaining: 256 << 10}
	command := exec.CommandContext(ctx, ffmpeg, "-hide_banner", argument) //nolint:gosec // Executable is installation configuration.
	command.Stdout, command.Stderr = &stdout, &stderr
	err := command.Run()
	return stdout.String() + "\n" + stderr.String(), err
}

type cappedBuffer struct {
	bytes.Buffer
	remaining int
}

func (buffer *cappedBuffer) Write(value []byte) (int, error) {
	size := len(value)
	value = value[:min(len(value), buffer.remaining)]
	_, _ = buffer.Buffer.Write(value)
	buffer.remaining -= len(value)
	return size, nil
}

func canOpenHardwareDevice(path string) bool {
	device, err := os.OpenFile(path, os.O_RDWR, 0) //nolint:gosec // Paths are fixed GPU device nodes or explicit installation configuration.
	if err != nil {
		return false
	}
	return device.Close() == nil
}

func accessibleHardwareDeviceKinds(paths []string, canOpen func(string) bool) map[string]string {
	result := make(map[string]string)
	for _, path := range paths {
		if kind := hardwareDeviceKind(path); kind != "" {
			if result[kind] == "" && canOpen(path) {
				result[kind] = path
			}
			continue
		}
		entries, err := os.ReadDir(path)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			child := filepath.Join(path, entry.Name())
			if kind := hardwareDeviceKind(child); kind != "" && result[kind] == "" && canOpen(child) {
				result[kind] = child
			}
		}
	}
	return result
}

func hardwareDeviceKind(path string) string {
	lower, base := strings.ToLower(filepath.ToSlash(path)), strings.ToLower(filepath.Base(path))
	switch base {
	case "mpp_service":
		return "mpp"
	case "rga":
		return "rga"
	}
	switch {
	case strings.HasPrefix(base, "renderd"): //nolint:misspell // Linux DRM device nodes use the renderD prefix.
		return "render"
	case strings.HasPrefix(base, "nvidia"):
		return "nvidia"
	case strings.Contains(lower, "/dma_heap/"):
		return "dma_heap"
	case videoDeviceName(base):
		return "video"
	default:
		return ""
	}
}

func videoDeviceName(base string) bool {
	if !strings.HasPrefix(base, "video") {
		return false
	}
	suffix := base[len("video"):]
	switch suffix {
	case "":
		return false
	default:
		return strings.Trim(suffix, "0123456789") == ""
	}
}

func linuxHardwarePlatform() string {
	return linuxHardwarePlatformWith(os.ReadFile)
}

func linuxHardwarePlatformWith(read func(string) ([]byte, error)) string {
	for _, path := range []string{"/proc/device-tree/compatible", "/sys/firmware/devicetree/base/compatible"} {
		value, err := read(path)
		if err == nil && strings.Contains(strings.ToLower(string(value)), "apple,") {
			return "asahi"
		}
	}
	return ""
}
