package dlna

import (
	"testing"

	"github.com/MikeO7/kinosail/packages/library"
)

func TestDLNAMediaTypesCoverScannedFormats(t *testing.T) {
	t.Parallel()
	for container, want := range map[string]string{
		"M2TS": "video/mp2t", "MKV": "video/x-matroska", "MP4": "video/mp4", "WEBM": "video/webm", "WMV": "video/x-ms-wmv",
		"AAC": "audio/aac", "AIF": "audio/aiff", "AIFF": "audio/aiff", "M4B": "audio/mp4", "MP3": "audio/mpeg", "OGA": "audio/ogg", "WEBA": "audio/webm",
		"AVIF": "image/avif", "BMP": "image/bmp", "JPG": "image/jpeg", "WEBP": "image/webp",
	} {
		if got := MediaType(library.Item{Container: container}); got != want {
			t.Errorf("mediaType(%q) = %q, want %q", container, got, want)
		}
	}
	if got := MediaType(library.Item{Container: "UNKNOWN"}); got != "application/octet-stream" {
		t.Fatalf("unknown media type = %q", got)
	}
}
