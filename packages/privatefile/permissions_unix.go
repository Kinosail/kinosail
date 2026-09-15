//go:build !windows

package privatefile

import "os"

func securePermissions(info os.FileInfo) bool {
	return info.Mode().Perm()&0o077 == 0
}

func syncDirectory(path string) error {
	directory, err := os.Open(path) //nolint:gosec // Callers pass installation-owned paths.
	if err != nil {
		return err
	}
	defer directory.Close()
	return directory.Sync()
}
