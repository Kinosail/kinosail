package transcodepolicy

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/MikeO7/kinosail/packages/isobmff"
	"github.com/MikeO7/kinosail/packages/workload"
)

type CheckResult struct {
	Status          string `json:"status"`
	Stage           string `json:"stage,omitempty"`
	Codec           string `json:"codec"`
	CodecName       string `json:"codecName"`
	Accelerator     string `json:"accelerator"`
	Backend         string `json:"backend"`
	Message         string `json:"message"`
	HardwareDecode  bool   `json:"hardwareDecode"`
	DecodeInput     string `json:"decodeInput,omitempty"`
	HardwareToneMap string `json:"hardwareToneMap,omitempty"`
	ToneMapInput    string `json:"toneMapInput,omitempty"`
	OutputHDR       string `json:"outputHDR,omitempty"`
	DurationMS      int64  `json:"durationMs,omitempty"`
}

func Check(ctx context.Context, ffmpeg string, options Settings, backend string) CheckResult {
	return runCheck(ctx, ffmpeg, options, backend, func() (string, error) { return os.MkdirTemp("", "kinosail-transcoder-check-") }, runFFmpeg)
}

func PendingCheck(options Settings, backend string) CheckResult {
	return CheckResult{Status: "not-run", Codec: options.Codec, CodecName: Definition(options.Codec).Name, Accelerator: options.Accelerator, Backend: checkBackend(options, backend)}
}

func runCheck(ctx context.Context, ffmpeg string, options Settings, backend string, temporary func() (string, error), run func(context.Context, string, ...string) error) CheckResult {
	if options.HardwareToneMap == "" {
		options.ToneMap = false
	}
	backend = checkBackend(options, backend)
	result := CheckResult{Status: "failed", Codec: options.Codec, CodecName: Definition(options.Codec).Name, Accelerator: options.Accelerator, Backend: backend}
	if message := checkSetup(ffmpeg, options); message != "" {
		result.Stage, result.Message = "setup", message
		return result
	}
	started := time.Now()
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	directory, err := temporary()
	if err != nil {
		result.Stage, result.Message = "setup", "Kinosail could not prepare the local transcoder check."
		return result
	}
	defer os.RemoveAll(directory)
	source, candidates := checkSources(directory, options)
	if err = runFirst(ctx, ffmpeg, candidates, run); err != nil {
		result.Stage, result.Message = "source", checkSourceMessage(options)
		return result
	}
	input, output := VideoArguments(options, "320")
	arguments := append([]string{"-hide_banner", "-loglevel", "error", "-y"}, input...)
	arguments = append(arguments, "-i", source, "-map", "0:v:0", "-map", "0:a:0", "-sn")
	arguments = append(arguments, output...)
	playlist := filepath.Join(directory, "index.m3u8")
	arguments = append(arguments, "-c:a", "aac", "-threads", "1", "-frames:v", "24", "-shortest", "-f", "hls", "-hls_time", "1", "-hls_segment_type", "fmp4", "-hls_fmp4_init_filename", "init.mp4", "-hls_segment_filename", filepath.Join(directory, "segment-%d.m4s"), playlist)
	if err = run(ctx, ffmpeg, arguments...); err != nil {
		result.Stage, result.Message = "encode", backend+" could not encode the local test clip. Choose Automatic or Software, save, and test again. If you selected hardware, make sure its GPU device is available to the Server container."
		return result
	}
	if !validCheckOutput(filepath.Join(directory, "init.mp4"), options) {
		result.Stage, result.Message = "output", "The encoder did not produce the requested playable MP4 format."
		return result
	}
	if err = run(ctx, ffmpeg, "-v", "error", "-i", playlist, "-frames:v", "1", "-f", "null", "-"); err != nil {
		result.Stage, result.Message = "decode", "FFmpeg could not decode the generated HLS presentation."
		return result
	}
	if options.HardwareToneMap != "" && !checkToneMapPixels(ctx, ffmpeg, directory, source, playlist, run) {
		result.Stage, result.Message = "tone-map", "The hardware path did not demonstrate HDR-to-SDR pixel conversion."
		return result
	}
	result.HardwareToneMap, result.ToneMapInput = options.HardwareToneMap, options.ToneMapInput
	result.HardwareDecode, result.DecodeInput, result.OutputHDR, result.DurationMS = options.HardwareDecode, options.DecodeInput, options.OutputHDR, time.Since(started).Milliseconds()
	result.Status, result.Message = "passed", "Kinosail encoded and decoded a local HLS clip with "+backend+". This is a smoke check, not a device or performance certification."
	return result
}

func runFirst(ctx context.Context, ffmpeg string, candidates [][]string, run func(context.Context, string, ...string) error) error {
	err := errors.New("no source encoder is available")
	for _, arguments := range candidates {
		if err = run(ctx, ffmpeg, arguments...); err == nil || ctx.Err() != nil {
			return err
		}
	}
	return err
}

func checkSourceMessage(options Settings) string {
	switch {
	case options.HardwareDecode && options.DecodeInput != "":
		return "FFmpeg could not create the local " + Definition(decodeInputCodec(options.DecodeInput)).Name + " test clip."
	case options.HardwareToneMap != "":
		return "FFmpeg could not create the local HDR test clip."
	default:
		return "FFmpeg could not create the local test clip. Check that FFmpeg includes H.264 and AAC."
	}
}

func checkBackend(options Settings, backend string) string {
	if options.Accelerator == "none" && options.Encoder != "" {
		return "Software (" + options.Encoder + ")"
	}
	return backend
}

func runFFmpeg(ctx context.Context, ffmpeg string, arguments ...string) error {
	return workload.RunContext(ctx, exec.CommandContext(ctx, ffmpeg, arguments...))
}

func checkSetup(ffmpeg string, options Settings) string {
	if ffmpeg == "" || !ValidCodec(options.Codec) || options.Codec == "auto" || options.Encoder == "" {
		return "Choose an available video encoder before running the check."
	}
	if options.HardwareToneMap != "" && (!options.ToneMap || options.DisableHardwareToneMap || options.OutputHDR != "" || !validToneMapMethod(options) || options.ToneMapInput != "hdr10" && options.ToneMapInput != "hlg") {
		return "The requested hardware tone mapping operation is unsupported."
	}
	if options.HardwareToneMap != "" && options.HardwareDecode && !HardwareDecodedToneMap(options) {
		return "The requested hardware tone mapping operation is unsupported."
	}
	if options.OutputHDR != "" && (options.Codec != "hevc" || options.OutputHDR != "hdr10" && options.OutputHDR != "hlg") {
		return "The requested output color format is unsupported."
	}
	return checkDecodeSetup(options)
}

func checkDecodeSetup(options Settings) string {
	if options.DecodeInput != "" && !ValidDecodeInput(options.DecodeInput) {
		return "The requested decode input is unsupported."
	}
	if !options.HardwareDecode {
		return ""
	}
	_, depth := DecodeInputCodec(options.DecodeInput)
	if depth == 0 || !HardwareDecodeInput(options.Accelerator) {
		return "The requested hardware decoding operation is unsupported."
	}
	if (options.HardwareToneMap != "" || options.OutputHDR != "") && depth != 10 {
		return "The requested hardware decoding operation is unsupported."
	}
	return ""
}

func validCheckOutput(path string, options Settings) bool {
	initialization, outputError := isobmff.Read(path)
	video, hasVideo := initialization.Video()
	expectedRange, expectedDepth := checkOutputColor(options.OutputHDR)
	return outputError == nil && isobmff.HasCodec(initialization, options.Codec) && hasVideo && video.Range == expectedRange && video.BitDepth == expectedDepth && video.Width == 320 && video.Height == 180
}

func checkOutputColor(hdr string) (string, int) {
	expectedRange, expectedDepth := "SDR", 8
	if hdr != "" {
		expectedRange, expectedDepth = "PQ", 10
		if hdr == "hlg" {
			expectedRange = "HLG"
		}
	}
	return expectedRange, expectedDepth
}
