//go:build linux

package catalog

import "syscall"

func networkFilesystem(path string) bool {
	var status syscall.Statfs_t
	if syscall.Statfs(path, &status) != nil {
		return false
	}
	switch uint32(status.Type) { //nolint:gosec // Filesystem magic numbers occupy the low 32 bits on every Linux architecture.
	case 0x6969, 0x517b, 0xff534d42, 0xfe534d42, 0x65735546, 0x01021997, 0x00c36400, 0x6a656a63:
		return true
	default:
		return false
	}
}
