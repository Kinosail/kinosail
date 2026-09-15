package server_test

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

func devToolsPort(profile string, timeout time.Duration) (int, bool) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		data, err := os.ReadFile(filepath.Join(profile, "DevToolsActivePort"))
		if err == nil {
			port, parseErr := strconv.Atoi(strings.Split(string(data), "\n")[0])
			if parseErr == nil {
				return port, true
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	return 0, false
}
