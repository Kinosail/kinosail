package appcli

import (
	"bufio"
	"io"
	"io/fs"
	"log/slog"
	"math"
	"path"
	"runtime/debug"
	"strconv"
	"strings"
)

const (
	serverMemoryShare  = 0.4
	gatewayMemoryShare = 0.8
	unlimitedCgroup    = int64(1) << 60
)

func applyMemoryLimit(getenv func(string) string, system fs.FS, share float64) {
	limit, source := memoryLimit(getenv, system, share)
	switch source {
	case "":
		slog.Debug("Go memory limit not configured", "reason", "no cgroup memory limit")
	case "cgroup":
		debug.SetMemoryLimit(limit)
		slog.Info("Go memory limit configured", "source", source, "limit_bytes", limit, "share", share)
	default:
		slog.Info("Go memory limit configured", "source", source, "limit_bytes", limit)
	}
}

func memoryLimit(getenv func(string) string, system fs.FS, share float64) (int64, string) {
	if getenv("GOMEMLIMIT") != "" {
		return debug.SetMemoryLimit(-1), "GOMEMLIMIT"
	}
	cgroup, found := cgroupMemoryLimit(system)
	if !found {
		return 0, ""
	}
	return int64(math.Floor(float64(cgroup) * share)), "cgroup"
}

func cgroupMemoryLimit(system fs.FS) (int64, bool) {
	membership, err := system.Open("proc/self/cgroup")
	if err != nil {
		return 0, false
	}
	defer membership.Close()
	unified, unifiedFound := "", false
	lines := bufio.NewScanner(io.LimitReader(membership, 64<<10))
	for lines.Scan() {
		fields := strings.SplitN(lines.Text(), ":", 3)
		if len(fields) != 3 {
			continue
		}
		if fields[0] == "0" && fields[1] == "" {
			unified, unifiedFound = fields[2], true
			continue
		}
		for _, controller := range strings.Split(fields[1], ",") {
			if controller == "memory" {
				return smallestLimit(system, "sys/fs/cgroup/memory", fields[2], "memory.limit_in_bytes")
			}
		}
	}
	if !unifiedFound {
		return 0, false
	}
	return smallestLimit(system, "sys/fs/cgroup", unified, "memory.max")
}

func smallestLimit(system fs.FS, root, group, file string) (int64, bool) {
	group = strings.Trim(group, "/")
	if group == ".." || strings.HasPrefix(group, "../") || strings.Contains(group, "/../") || !fs.ValidPath(path.Join(root, group)) {
		group = ""
	}
	smallest, found := int64(0), false
	for {
		if limit, ok := readLimit(system, path.Join(root, group, file)); ok && (!found || limit < smallest) {
			smallest, found = limit, true
		}
		if group == "" {
			return smallest, found
		}
		group = strings.TrimSuffix(path.Dir(group), ".")
	}
}

func readLimit(system fs.FS, name string) (int64, bool) {
	contents, err := fs.ReadFile(system, name)
	if err != nil || len(contents) > 64 {
		return 0, false
	}
	limit, err := strconv.ParseInt(strings.TrimSpace(string(contents)), 10, 64)
	return limit, err == nil && limit > 0 && limit < unlimitedCgroup
}
