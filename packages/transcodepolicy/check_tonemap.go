package transcodepolicy

import (
	"context"
	"io"
	"os"
	"path/filepath"
)

// A mapper must change decoded luminance, not merely stamp SDR metadata onto
// HDR pixels. This is a no-op guard, not a perceptual color-quality metric.
func checkToneMapPixels(ctx context.Context, ffmpeg, directory, source, output string, run func(context.Context, string, ...string) error) bool {
	var samples [][]byte
	for index, path := range []string{source, output} {
		name := "input.raw"
		if index == 1 {
			name = "output.raw"
		}
		target := filepath.Join(directory, name)
		if run(ctx, ffmpeg, "-hide_banner", "-loglevel", "error", "-y", "-i", path, "-map", "0:v:0", "-vf", "scale=32:18,format=gray", "-frames:v", "1", "-f", "rawvideo", target) != nil {
			return false
		}
		sample, err := readToneMapSample(target)
		if err != nil || len(sample) != 32*18 {
			return false
		}
		samples = append(samples, sample)
	}
	return changedToneMapPixels(samples[0], samples[1])
}

func readToneMapSample(path string) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return io.ReadAll(io.LimitReader(file, 32*18+1))
}

func changedToneMapPixels(input, output []byte) bool {
	if len(input) != 32*18 || len(output) != len(input) {
		return false
	}
	difference, low, high := 0, 255, 0
	for index, value := range output {
		delta := int(value) - int(input[index])
		if delta < 0 {
			delta = -delta
		}
		difference += delta
		low = min(low, int(value))
		high = max(high, int(value))
	}
	return difference > 3*len(input) && high-low > 20
}
