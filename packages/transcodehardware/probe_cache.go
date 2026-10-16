package transcodehardware

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/MikeO7/kinosail/packages/transcodepolicy"
)

const (
	probeCacheVersion = 1
	probeCacheLimit   = 1 << 20
)

type probeCache struct {
	Version  int             `json:"version"`
	Key      string          `json:"key"`
	Selected string          `json:"selected"`
	Backends []cachedBackend `json:"backends"`
}

type cachedBackend struct {
	ID         string            `json:"id"`
	Name       string            `json:"name"`
	Encoder    string            `json:"encoder"`
	Device     string            `json:"device"`
	Status     string            `json:"status"`
	Reason     string            `json:"reason"`
	Action     string            `json:"action"`
	Supported  bool              `json:"supported"`
	Detected   bool              `json:"detected"`
	Usable     bool              `json:"usable"`
	Relevant   bool              `json:"relevant"`
	Encoders   map[string]string `json:"encoders"`
	Operations []cachedOperation `json:"operations"`
}

type cachedOperation struct {
	Operation
	Device   string `json:"device"`
	Identity string `json:"identity"`
}

func probeKey(options ProbeOptions) string {
	options = probePlatform(options)
	ffmpeg := options.FFmpeg
	if path, err := exec.LookPath(ffmpeg); err == nil {
		ffmpeg = path
	}
	if absolute, err := filepath.Abs(ffmpeg); err == nil {
		ffmpeg = absolute
	}
	var key strings.Builder
	fmt.Fprintf(&key, "%d\x00%s\x00%s\x00%s\x00%s", probeCacheVersion, options.GOOS, options.GOARCH, options.Application, ffmpeg)
	if info, err := os.Stat(ffmpeg); err == nil {
		fmt.Fprintf(&key, "\x00%d\x00%d", info.Size(), info.ModTime().UnixNano())
	}
	for _, device := range accessibleHardwareDevices(hardwareDevices(options.Devices)) {
		key.WriteString("\x00" + device)
	}
	return fmt.Sprintf("%x", sha256.Sum256([]byte(key.String())))
}

func StoreProbe(path string, options ProbeOptions, capabilities Capabilities) error {
	if capabilities.verification == nil || capabilities.Probing || !options.Enabled || options.FFmpeg == "" {
		return errors.New("only verified hardware evidence can be cached")
	}
	cache := probeCache{Version: probeCacheVersion, Key: probeKey(options), Selected: capabilities.Selected}
	state := capabilities.verification
	state.mu.RLock()
	for _, backend := range capabilities.Backends {
		entry := cachedBackend{ID: backend.ID, Name: backend.Name, Encoder: backend.Encoder, Device: backend.Device, Status: backend.Status, Reason: backend.Reason, Action: backend.Action, Supported: backend.Supported, Detected: backend.Detected, Usable: backend.Usable, Relevant: backend.Relevant, Encoders: backend.encoders}
		for _, operation := range state.operations[backend.ID] {
			entry.Operations = append(entry.Operations, cachedOperation{Operation: operation, Device: operation.Device, Identity: operation.identity})
		}
		cache.Backends = append(cache.Backends, entry)
	}
	state.mu.RUnlock()
	data, err := json.Marshal(cache)
	if err != nil {
		return err
	}
	temporary, err := os.CreateTemp(filepath.Dir(path), ".hardware-probe-*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(temporary.Name())
	if _, err = temporary.Write(data); err == nil {
		err = temporary.Sync()
	}
	if closeErr := temporary.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	return os.Rename(temporary.Name(), path)
}

func CachedProbe(path string, options ProbeOptions) (Capabilities, bool) {
	if !options.Enabled || options.FFmpeg == "" {
		return Capabilities{}, false
	}
	file, err := os.Open(path)
	if err != nil {
		return Capabilities{}, false
	}
	data, err := io.ReadAll(io.LimitReader(file, probeCacheLimit+1))
	_ = file.Close()
	var cache probeCache
	if err != nil || len(data) > probeCacheLimit || json.Unmarshal(data, &cache) != nil || cache.Version != probeCacheVersion || cache.Key != probeKey(options) {
		return Capabilities{}, false
	}
	return restoreProbe(cache, probePlatform(options))
}

func restoreProbe(cache probeCache, options ProbeOptions) (Capabilities, bool) {
	known, primary := map[string]bool{"none": true}, map[string]string{}
	for _, definition := range hardwareDefinitions(options.GOOS, options.GOARCH, func(...string) bool { return true }, "") {
		known[definition.id], primary[definition.id] = true, definition.encoder
	}
	devices := accessibleHardwareDevices(hardwareDevices(options.Devices))
	state := &verification{ffmpeg: options.FFmpeg, operations: make(map[string][]Operation), failed: make(map[string]time.Time)}
	capabilities := Capabilities{Selected: cache.Selected, Probed: true, verification: state}
	if len(cache.Backends) > len(known) || !known[cache.Selected] {
		return Capabilities{}, false
	}
	for _, entry := range cache.Backends {
		validPrimary := entry.Encoder == primary[entry.ID] || entry.ID == "none" && (entry.Encoder == "" || knownEncoder("none", "h264", entry.Encoder))
		if !known[entry.ID] || !validPrimary || state.operations[entry.ID] != nil || !validCachedBackend(entry, devices) {
			return Capabilities{}, false
		}
		state.operations[entry.ID] = []Operation{}
		for _, cached := range entry.Operations {
			operation := cached.Operation
			operation.Device, operation.identity = cached.Device, cached.Identity
			state.operations[entry.ID] = append(state.operations[entry.ID], operation)
		}
		encoders := make(map[string]string, len(entry.Encoders))
		for codec, encoder := range entry.Encoders {
			encoders[codec] = encoder
		}
		backend := Backend{ID: entry.ID, Name: entry.Name, Encoder: entry.Encoder, Device: entry.Device, Supported: entry.Supported, Detected: entry.Detected, Usable: entry.Usable, Relevant: entry.Relevant, Status: entry.Status, Reason: entry.Reason, Action: entry.Action, encoders: encoders, verification: state}
		backend.Codecs = transcodepolicy.EncoderCodecIDs(encoders)
		backend.Operations = append([]Operation(nil), state.operations[entry.ID]...)
		capabilities.Backends = append(capabilities.Backends, backend)
	}
	capabilities.Codecs = codecCapabilities(capabilities)
	return capabilities, len(capabilities.Backends) > 0 && capabilities.Backends[0].ID == "none"
}

func validCachedBackend(entry cachedBackend, devices []string) bool {
	allowed := map[string]bool{"": true}
	for _, device := range devices {
		allowed[device] = true
		allowed[backendDevice(entry.ID, device)] = true
	}
	if !allowed[entry.Device] || len(entry.Operations) > 256 || len(entry.Encoders) > 8 || !boundedText(entry.Name, entry.Encoder, entry.Status, entry.Reason, entry.Action) {
		return false
	}
	for codec, encoder := range entry.Encoders {
		if !knownEncoder(entry.ID, codec, encoder) {
			return false
		}
	}
	for _, operation := range entry.Operations {
		if !allowed[operation.Device] || !validCachedOperation(entry.ID, operation) {
			return false
		}
	}
	return true
}

func validCachedOperation(backend string, operation cachedOperation) bool {
	method := operation.HardwareToneMap == "" || slices.Contains(transcodepolicy.ToneMapMethods(backend, operation.Device), operation.HardwareToneMap)
	if len(operation.DecodeInputs) > len(transcodepolicy.DecodeInputs()) || slices.ContainsFunc(operation.DecodeInputs, func(input string) bool { return !transcodepolicy.ValidDecodeInput(input) }) {
		return false
	}
	return method && transcodepolicy.ValidCodec(operation.Codec) && operation.Codec != "auto" && operation.Codec != "" &&
		slices.Contains([]string{"passed", "failed"}, operation.Status) && slices.Contains([]string{"", "hdr10", "hlg"}, operation.OutputHDR) &&
		slices.Contains([]string{"", "hdr10", "hlg"}, operation.ToneMapInput) && len(operation.Identity) == 64 && strings.Trim(operation.Identity, "0123456789abcdef") == ""
}

func knownEncoder(backend, codec, encoder string) bool {
	definition := transcodepolicy.Definition(codec)
	if definition.ID != codec {
		return false
	}
	if backend == "none" {
		return slices.Contains(definition.Software, encoder)
	}
	return definition.Hardware[backend] == encoder && encoder != ""
}

func boundedText(values ...string) bool {
	for _, value := range values {
		if len(value) > 512 || strings.ContainsRune(value, '\x00') {
			return false
		}
	}
	return true
}
