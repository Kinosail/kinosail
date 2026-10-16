//go:build linux

package catalog

import (
	"bufio"
	"bytes"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"syscall"
)

const (
	maximumMountInfo     = 4 << 20
	maximumNestedMounts  = 256
	mountInfoMountColumn = 4
)

func networkFilesystem(path string) bool {
	var status syscall.Statfs_t
	if syscall.Statfs(path, &status) != nil {
		return false
	}
	switch uint32(status.Type) { //nolint:gosec // Filesystem magic numbers occupy the low 32 bits on every Linux architecture.
	case 0x6969, 0x517b, 0xff534d42, 0xfe534d42, 0x65735546, 0x01021997, 0x00c36400, 0x6a656a63,
		0x0bd00bd0, 0x47504653, 0x19830326, 0x5346414f, 0x6b414653, 0x7461636f, 0x73757245, 0x786f4256:
		return true
	default:
		return false
	}
}

func nestedNetworkMounts(roots []ScanRoot) map[ScanRoot][]string {
	file, err := os.Open("/proc/self/mountinfo")
	if err != nil {
		return nil
	}
	defer file.Close()
	mountinfo, err := io.ReadAll(io.LimitReader(file, maximumMountInfo))
	if err != nil {
		return nil
	}
	nested := make(map[ScanRoot][]string)
	for _, root := range roots {
		if mounts := networkMountsBelow(root.Path, bytes.NewReader(mountinfo), networkFilesystem); len(mounts) > 0 {
			nested[root] = mounts
		}
	}
	return nested
}

func networkMountsBelow(root string, mountinfo io.Reader, network func(string) bool) []string {
	resolved, err := filepath.EvalSymlinks(root)
	if err != nil {
		return nil
	}
	var below []string
	lines := bufio.NewScanner(mountinfo)
	for lines.Scan() && len(below) < maximumNestedMounts {
		fields := strings.Fields(lines.Text())
		if len(fields) <= mountInfoMountColumn {
			continue
		}
		mount := unescapeMountPath(fields[mountInfoMountColumn])
		if relative, err := filepath.Rel(resolved, mount); err == nil && relative != "." && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			below = append(below, mount)
		}
	}
	slices.Sort(below)
	var selected []string
	for _, mount := range slices.Compact(below) {
		covered := slices.ContainsFunc(selected, func(parent string) bool { return strings.HasPrefix(mount, parent+string(filepath.Separator)) })
		if !covered && network(mount) {
			selected = append(selected, mount)
		}
	}
	for position, mount := range selected {
		relative, _ := filepath.Rel(resolved, mount)
		selected[position] = filepath.Join(root, relative)
	}
	return selected
}

func unescapeMountPath(value string) string {
	var path strings.Builder
	for position := 0; position < len(value); position++ {
		if value[position] == '\\' && position+3 < len(value) {
			if code, err := strconv.ParseUint(value[position+1:position+4], 8, 8); err == nil {
				path.WriteByte(byte(code))
				position += 3
				continue
			}
		}
		path.WriteByte(value[position])
	}
	return path.String()
}
