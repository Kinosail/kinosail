package servertest

import (
	"testing"
)

func AssertPlayerFormattingUsesStableHumanReadableBoundaries(t *testing.T, byteSize func(int64) string, oneOf func(string, ...string) bool) {
	AssertByteSizeBoundaries(t, byteSize)
	if !oneOf("hevc", "h264", "hevc") || oneOf("vp9", "h264", "hevc") {
		t.Fatal("oneOf classification is incorrect")
	}
}

func AssertByteSizeBoundaries(t *testing.T, byteSize func(int64) string) {
	for size, want := range map[int64]string{0: "0 B", 1024: "1.0 KB", 1024 * 1024: "1.0 MB", 1024 * 1024 * 1024: "1.0 GB"} {
		if got := byteSize(size); got != want {
			t.Errorf("byteSize(%d) = %q, want %q", size, got, want)
		}
	}
}
