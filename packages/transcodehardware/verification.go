package transcodehardware

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/MikeO7/kinosail/packages/transcodepolicy"
)

type Operation struct {
	ToneMapSoftwareFrames bool `json:"toneMapSoftwareFrames,omitempty"`
	identity              string
	Codec                 string `json:"codec"`
	Device                string `json:"-"`
	OutputHDR             string `json:"outputHDR,omitempty"`
	HardwareToneMap       string `json:"hardwareToneMap,omitempty"`
	ToneMapInput          string `json:"toneMapInput,omitempty"`
	DecodeH264            bool   `json:"decodeH264"`
	DecodeHEVC            bool   `json:"decodeHEVC,omitempty"`
	DecodeHEVC10          bool   `json:"decodeHEVC10,omitempty"`
	DecodeAV1             bool   `json:"decodeAV1,omitempty"`
	ToneMapHardwareFrames bool   `json:"toneMapHardwareFrames,omitempty"`
	Status                string `json:"status"`
}

type verification struct {
	ffmpeg     string
	mu         sync.RWMutex
	operations map[string][]Operation
	failed     map[string]time.Time
}

func operationKey(backend, device, codec string) string {
	return backend + "\x00" + device + "\x00" + codec
}

func (state *verification) operationsFor(backend, codec string) []Operation {
	state.mu.RLock()
	defer state.mu.RUnlock()
	var result []Operation
	for _, operation := range state.operations[backend] {
		if operation.Codec == codec && operation.Status == "passed" && (state.ffmpeg == "" || operation.identity == operationIdentity(state.ffmpeg, operation.Device)) && time.Now().After(state.failed[operationKey(backend, operation.Device, codec)]) {
			result = append(result, operation)
		}
	}
	return result
}

func (backend Backend) operation(codec, hdr string) (Operation, bool) {
	if backend.verification == nil {
		return Operation{Codec: codec, Device: backend.Device}, backend.Usable && backend.encoders[codec] != "" && hdr == ""
	}
	for _, operation := range backend.verification.operationsFor(backend.ID, codec) {
		if operation.OutputHDR == hdr && operation.HardwareToneMap == "" {
			return operation, true
		}
	}
	return Operation{}, false
}

func (capabilities Capabilities) RecordFailure(options transcodepolicy.Settings) {
	if capabilities.verification == nil {
		return
	}
	state := capabilities.verification
	state.mu.Lock()
	state.failed[operationKey(options.Accelerator, options.Device, options.Codec)] = time.Now().Add(10 * time.Minute)
	state.mu.Unlock()
}

func (capabilities Capabilities) RecordDecodeFailure(options transcodepolicy.Settings) {
	if capabilities.verification == nil {
		return
	}
	state := capabilities.verification
	state.mu.Lock()
	defer state.mu.Unlock()
	for index := range state.operations[options.Accelerator] {
		operation := &state.operations[options.Accelerator][index]
		if operation.Device == options.Device && operation.Codec == options.Codec {
			switch options.DecodeInput {
			case "hevc":
				operation.DecodeHEVC = false
			case "hevc10":
				operation.DecodeHEVC10 = false
			case "av1":
				operation.DecodeAV1 = false
			default:
				operation.DecodeH264 = false
			}
		}
	}
}

func (capabilities Capabilities) RecordCheck(options transcodepolicy.Settings, result transcodepolicy.CheckResult) {
	if capabilities.verification == nil {
		return
	}
	if result.Status != "passed" {
		capabilities.RecordProcessingFailure(options)
		return
	}
	if result.HardwareToneMap != options.HardwareToneMap || result.HardwareToneMap != "" && result.ToneMapInput != options.ToneMapInput {
		return
	}
	state := capabilities.verification
	state.mu.Lock()
	defer state.mu.Unlock()
	delete(state.failed, operationKey(options.Accelerator, options.Device, options.Codec))
	operation := Operation{identity: operationIdentity(state.ffmpeg, options.Device), Codec: options.Codec, Device: options.Device, OutputHDR: options.OutputHDR, HardwareToneMap: result.HardwareToneMap, ToneMapInput: result.ToneMapInput, ToneMapSoftwareFrames: options.HardwareToneMap != "" && transcodepolicy.ToneMapSoftwareFrames(options), ToneMapHardwareFrames: options.HardwareToneMap != "" && transcodepolicy.ToneMapHardwareFrames(options), DecodeH264: result.HardwareDecode && options.DecodeInput == "", Status: "passed"}
	for index, previous := range state.operations[options.Accelerator] {
		if previous.Codec == operation.Codec && previous.Device == operation.Device && previous.OutputHDR == operation.OutputHDR && previous.HardwareToneMap == operation.HardwareToneMap && previous.ToneMapInput == operation.ToneMapInput && previous.ToneMapSoftwareFrames == operation.ToneMapSoftwareFrames && previous.ToneMapHardwareFrames == operation.ToneMapHardwareFrames {
			state.operations[options.Accelerator][index] = operation
			return
		}
	}
	state.operations[options.Accelerator] = append(state.operations[options.Accelerator], operation)
}

func verify(ctx context.Context, options ProbeOptions, capabilities Capabilities, check func(context.Context, string, transcodepolicy.Settings, string) transcodepolicy.CheckResult) Capabilities {
	if !options.Enabled || options.FFmpeg == "" {
		return capabilities
	}
	parent := ctx
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	state := &verification{ffmpeg: options.FFmpeg, operations: make(map[string][]Operation), failed: make(map[string]time.Time)}
	capabilities.verification = state
	capabilities.Selected = "none"
	devices := accessibleHardwareDevices(hardwareDevices(options.Devices))
	eligible := make(map[string]bool)
	for index := range capabilities.Backends {
		backend := &capabilities.Backends[index]
		eligible[backend.ID] = backend.Usable
		backend.verification, backend.Usable = state, false
	}
	state.verifyBaselines(ctx, options, capabilities.Backends, devices, eligible, check)
	eachBackend(capabilities.Backends, func(backend *Backend) { state.verifyOptionalFormats(ctx, options, backend, check) })
	for index := range capabilities.Backends {
		backend := &capabilities.Backends[index]
		backend.Operations = state.snapshot(backend.ID)
		if !eligible[backend.ID] {
			continue
		}
		backend.Status, backend.Reason = "Not verified", "No successful encoding operation was established within the startup check budget."
		if backend.Usable {
			backend.Status, backend.Reason = "Smoke check passed", "FFmpeg encoded and decoded a synthetic HLS clip. Real media and device performance still require verification."
		}
		if capabilities.Selected == "none" && backend.ID != "none" && backend.Usable {
			capabilities.Selected = backend.ID
		}
	}
	state.verifyToneMaps(parent, options, capabilities.Backends, check)
	capabilities.Codecs = codecCapabilities(capabilities)
	return capabilities
}

func accessibleHardwareDevices(paths []string) []string {
	var result []string
	for _, path := range paths {
		candidates := []string{path}
		if info, err := os.Stat(path); err == nil && info.IsDir() {
			entries, _ := os.ReadDir(path)
			candidates = nil
			for _, entry := range entries {
				candidates = append(candidates, filepath.Join(path, entry.Name()))
			}
		}
		for _, candidate := range candidates {
			if hardwareDeviceKind(candidate) != "" && canOpenHardwareDevice(candidate) && !slices.Contains(result, candidate) && len(result) < 32 {
				result = append(result, candidate)
			}
		}
	}
	slices.Sort(result)
	return result
}

func backendDevices(backend Backend, devices []string) []string {
	var result []string
	for _, device := range devices {
		if candidate := backendDevice(backend.ID, device); candidate != "" {
			result = append(result, candidate)
		}
	}
	if len(result) > 8 {
		result = result[:8]
	}
	if !slices.Contains([]string{"qsv", "vaapi", "cuda", "rkmpp"}, backend.ID) {
		return []string{""}
	}
	if len(result) == 0 && backend.Device == "" {
		return []string{""}
	}
	return result
}

func operationIdentity(ffmpeg, device string) string {
	if path, err := exec.LookPath(ffmpeg); err == nil {
		ffmpeg = path
	}
	var identity strings.Builder
	identity.WriteString(ffmpeg + "\x00" + device)
	for _, path := range []string{ffmpeg, device} {
		if info, err := os.Stat(path); err == nil {
			fmt.Fprint(&identity, info.Size(), info.ModTime().UnixNano(), info.Mode())
		}
	}
	for _, path := range []string{"/sys/module/i915/version", "/sys/module/amdgpu/version", "/sys/module/nvidia/version", "/proc/driver/nvidia/version"} {
		file, err := os.Open(path)
		if err != nil {
			continue
		}
		data, _ := io.ReadAll(io.LimitReader(file, 4096))
		_ = file.Close()
		identity.Write(data)
	}
	return fmt.Sprintf("%x", sha256.Sum256([]byte(identity.String())))
}

func backendDevice(backend, device string) string {
	kind := hardwareDeviceKind(device)
	if (backend == "vaapi" || backend == "qsv" || backend == "rkmpp") && kind == "render" {
		vendor, _ := os.ReadFile(filepath.Join("/sys/class/drm", filepath.Base(device), "device/vendor"))
		if backend == "qsv" && len(vendor) > 0 && strings.TrimSpace(string(vendor)) != "0x8086" {
			return ""
		}
		return device
	}
	if backend == "cuda" && kind == "nvidia" && strings.Trim(strings.TrimPrefix(filepath.Base(device), "nvidia"), "0123456789") == "" {
		return strings.TrimPrefix(filepath.Base(device), "nvidia")
	}
	return ""
}
