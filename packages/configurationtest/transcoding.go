package configurationtest

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func (configuration contract[Source, PublicValue, Snapshot]) testTranscodingAcceleratorValidationIsSharedAcrossConfigurationSources(t *testing.T) {
	t.Parallel()
	for _, accelerator := range []string{"rkmpp", "v4l2m2m", "amf", "mf"} {
		loaded, err := configuration.Load(t.TempDir(), "", func(name string) (string, bool) {
			return accelerator, name == "KINOSAIL_TRANSCODE_ACCELERATOR"
		})
		if err != nil || loaded.String("transcoding.accelerator") != accelerator {
			t.Errorf("accelerator %q = %q, %v", accelerator, loaded.String("transcoding.accelerator"), err)
		}
	}

	directory := t.TempDir()
	if err := configuration.Set(directory, "transcoding.accelerator", "rkmpp"); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(directory, "configuration.json")
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, accelerator := range []string{"rk-mpp", "unknown", strings.Repeat("a", 16<<10)} {
		if err := configuration.Set(directory, "transcoding.accelerator", accelerator); err == nil {
			t.Errorf("invalid accelerator %q was accepted", accelerator)
		}
		after, readErr := os.ReadFile(path)
		if readErr != nil || !bytes.Equal(after, before) {
			t.Fatalf("invalid accelerator changed configuration: after=%q err=%v", after, readErr)
		}
	}
}

func (configuration contract[Source, PublicValue, Snapshot]) testTranscodingCodecValidationIsSharedAcrossConfigurationSources(t *testing.T) {
	t.Parallel()
	for _, codec := range []string{"auto", "h264", "hevc", "av1", "vp9"} {
		loaded, err := configuration.Load(t.TempDir(), "", func(name string) (string, bool) {
			return codec, name == "KINOSAIL_TRANSCODE_CODEC"
		})
		if err != nil || loaded.String("transcoding.codec") != codec {
			t.Errorf("codec %q = %q, %v", codec, loaded.String("transcoding.codec"), err)
		}
	}
	directory := t.TempDir()
	if err := configuration.Set(directory, "transcoding.codec", "h264"); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(directory, "configuration.json")
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, codec := range []string{"vvc", "av2", "h265", "AV1", strings.Repeat("a", 16<<10)} {
		if err := configuration.Set(directory, "transcoding.codec", codec); err == nil {
			t.Errorf("invalid codec %q was accepted", codec)
		}
		after, readErr := os.ReadFile(path)
		if readErr != nil || !bytes.Equal(after, before) {
			t.Fatalf("invalid codec changed configuration: after=%q err=%v", after, readErr)
		}
	}
}
