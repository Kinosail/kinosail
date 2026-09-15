package configuration_test

import (
	"os"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail-subtitles/internal/configuration"
)

func TestLocalSubtitleConfigurationRejectsUnsafeValuesWithoutWriting(t *testing.T) {
	for key, values := range map[string][]string{
		"binaries.tesseract":                   {"", "-x", "../tesseract", "https://example.test/ocr", "tesseract\n", strings.Repeat("x", 4097)},
		"binaries.whisper":                     {"./whisper-cli", "whisper-cli\x00", " whisper-cli"},
		"subtitles.transcription_model":        {"model.bin", "https://example.test/model", "/models/model\x00", strings.Repeat("/", 4097)},
		"subtitles.transcription_architecture": {"", "unknown", "small;echo"},
	} {
		for _, value := range values {
			directory := t.TempDir()
			if err := configuration.Set(directory, key, value); err == nil {
				t.Errorf("accepted %s", key)
			}
			entries, err := os.ReadDir(directory)
			if err != nil || len(entries) != 0 {
				t.Errorf("invalid %s wrote configuration: %v/%v", key, entries, err)
			}
		}
	}
}
