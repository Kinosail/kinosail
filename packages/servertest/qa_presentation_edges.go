package servertest

import (
	"testing"
)

func AssertByteSizeBoundaries(t *testing.T, byteSize func(int64) string) {
	for size, want := range map[int64]string{0: "0 B", 1024: "1.0 KB", 1024 * 1024: "1.0 MB", 1024 * 1024 * 1024: "1.0 GB"} {
		if got := byteSize(size); got != want {
			t.Errorf("byteSize(%d) = %q, want %q", size, got, want)
		}
	}
}
