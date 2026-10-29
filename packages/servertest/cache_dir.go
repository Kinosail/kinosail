package servertest

import (
	"os"
	"testing"
	"time"
)

func CacheDir(t testing.TB) string {
	t.Helper()
	directory := t.TempDir()
	t.Cleanup(func() {
		quiet := 0
		for deadline := time.Now().Add(5 * time.Second); quiet < 3 && time.Now().Before(deadline); time.Sleep(20 * time.Millisecond) {
			if _, err := os.Stat(directory); os.IsNotExist(err) {
				quiet++
				continue
			}
			quiet = 0
			_ = os.RemoveAll(directory)
		}
	})
	return directory
}
