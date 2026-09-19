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

// Operation records an actual encoder/format check on one device. DecodeH264
// certifies only the tested 8-bit H.264 input, never all codecs on that backend.
type Operation struct {
	ToneMapSoftwareFrames bool `json:"toneMapSoftwareFrames,omitempty"`
	identity              string
	Codec                 string `json:"codec"`
	Device                string `json:"-"`
	OutputHDR             string `json:"outputHDR,omitempty"`
	HardwareToneMap       string `json:"hardwareToneMap,omitempty"`
	ToneMapInput          string `json:"toneMapInput,omitempty"`
	DecodeH264            bool   `json:"decodeH264"`
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

// RecordFailure temporarily removes a failed device/codec from selection. The
// snapshot remains immutable; current sessions can safely retain its evidence.
func (capabilities Capabilities) RecordFailure(options transcodepolicy.Settings) {
	if capabilities.verification == nil {
		return
	}
	state := capabilities.verification
	state.mu.Lock()
	state.failed[operationKey(options.Accelerator, options.Device, options.Codec)] = time.Now().Add(10 * time.Minute)
	state.mu.Unlock()
}

// RecordDecodeFailure retains verified encoding while disabling the failed
// hardware input path. An encode-only retry can still quarantine the operation.
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
			operation.DecodeH264 = false
		}
	}
}

// RecordCheck makes a successful explicit smoke check available to selection.
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
	operation := Operation{identity: operationIdentity(state.ffmpeg, options.Device), Codec: options.Codec, Device: options.Device, OutputHDR: options.OutputHDR, HardwareToneMap: result.HardwareToneMap, ToneMapInput: result.ToneMapInput, ToneMapSoftwareFrames: options.HardwareToneMap != "" && transcodepolicy.ToneMapSoftwareFrames(options), DecodeH264: result.HardwareDecode, Status: "passed"}
	for index, previous := range state.operations[options.Accelerator] {
		if previous.Codec == operation.Codec && previous.Device == operation.Device && previous.OutputHDR == operation.OutputHDR && previous.HardwareToneMap == operation.HardwareToneMap && previous.ToneMapInput == operation.ToneMapInput && previous.ToneMapSoftwareFrames == operation.ToneMapSoftwareFrames {
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
	// Establish the baseline on every candidate before spending the bounded
	// startup budget on more expensive codecs or optional decoding stages.
	state.verifyBaselines(ctx, options, capabilities.Backends, devices, eligible, check)
	for index := range capabilities.Backends {
		backend := &capabilities.Backends[index]
		state.verifyOptionalFormats(ctx, options, backend, check)
		backend.Operations = append([]Operation(nil), state.operations[backend.ID]...)
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

func verifyHardwareDecode(ctx context.Context, options ProbeOptions, settings transcodepolicy.Settings, backend *Backend, state *verification, check func(context.Context, string, transcodepolicy.Settings, string) transcodepolicy.CheckResult) {
	if !slices.Contains([]string{"qsv", "vaapi", "cuda", "rkmpp"}, backend.ID) || ctx.Err() != nil {
		return
	}
	settings.HardwareDecode = true
	probeCtx, stop := context.WithTimeout(ctx, 3*time.Second)
	defer stop()
	if result := check(probeCtx, options.FFmpeg, settings, backend.Name); result.Status == "passed" {
		operations := state.operations[backend.ID]
		for index := range operations {
			if operations[index].Device == settings.Device && operations[index].Codec == settings.Codec && operations[index].OutputHDR == "" {
				operations[index].DecodeH264 = true
			}
		}
	}
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

// Evidence is process-local and expires when the executable, device node or
// available kernel-driver identity changes. A restart always re-probes it.
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

func (state *verification) verifyBaselines(ctx context.Context, options ProbeOptions, backends []Backend, devices []string, eligible map[string]bool, check func(context.Context, string, transcodepolicy.Settings, string) transcodepolicy.CheckResult) {
	for _, codec := range []string{"h264", "hevc", "av1", "vp9"} {
		for index := range backends {
			backend := &backends[index]
			if !eligible[backend.ID] {
				continue
			}
			state.verifyCodec(ctx, options, backend, codec, devices, check)
		}
		if codec == "h264" {
			// Optional software codecs must not consume the startup budget
			// before the preferred hardware H.264 frame path is checked.
			for index := range backends {
				backend := &backends[index]
				state.verifyBaselineDecode(ctx, options, backend, codec, check)
			}
		}
	}
}
