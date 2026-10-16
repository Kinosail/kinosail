package appcli

import (
	"bytes"
	"log/slog"
	"runtime/debug"
	"strings"
	"testing"
	"testing/fstest"
)

func cgroupFiles(membership string, limits map[string]string) fstest.MapFS {
	files := fstest.MapFS{}
	if membership != "" {
		files["proc/self/cgroup"] = &fstest.MapFile{Data: []byte(membership)}
	}
	for path, value := range limits {
		files[path] = &fstest.MapFile{Data: []byte(value)}
	}
	return files
}

func TestMemoryLimitFollowsTheContainerCgroup(t *testing.T) {
	t.Parallel()
	for name, test := range map[string]struct {
		membership string
		limits     map[string]string
		share      float64
		want       int64
	}{
		"namespaced v2":           {"0::/\n", map[string]string{"sys/fs/cgroup/memory.max": "4294967296\n"}, 0.4, 1717986918},
		"nested v2 parent limit":  {"0::/kinosail.slice/app.scope\n", map[string]string{"sys/fs/cgroup/kinosail.slice/app.scope/memory.max": "max\n", "sys/fs/cgroup/kinosail.slice/memory.max": "268435456\n"}, 0.8, 214748364},
		"smallest v2 ancestor":    {"0::/outer/inner\n", map[string]string{"sys/fs/cgroup/outer/inner/memory.max": "1073741824\n", "sys/fs/cgroup/outer/memory.max": "2147483648\n"}, 0.5, 536870912},
		"v1 memory controller":    {"12:pids:/docker/abc\n4:cpu,memory:/docker/abc\n", map[string]string{"sys/fs/cgroup/memory/docker/abc/memory.limit_in_bytes": "268435456\n"}, 0.8, 214748364},
		"v1 namespaced root":      {"4:memory:/\n", map[string]string{"sys/fs/cgroup/memory/memory.limit_in_bytes": "1073741824"}, 0.4, 429496729},
		"hybrid uses v1 memory":   {"0::/\n4:memory:/job\n", map[string]string{"sys/fs/cgroup/memory/job/memory.limit_in_bytes": "1073741824", "sys/fs/cgroup/memory.max": "4096"}, 0.4, 429496729},
		"traversal uses own root": {"0::/../../host\n", map[string]string{"sys/fs/cgroup/memory.max": "1073741824", "sys/host/memory.max": "1024"}, 0.5, 536870912},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			limit, source := memoryLimit(func(string) string { return "" }, cgroupFiles(test.membership, test.limits), test.share)
			if limit != test.want || source != "cgroup" {
				t.Fatalf("limit = %d from %q, want %d from cgroup", limit, source, test.want)
			}
		})
	}
}

func TestMemoryLimitLeavesUnlimitedOrUnknownCgroupsAlone(t *testing.T) {
	t.Parallel()
	for name, files := range map[string]fstest.MapFS{
		"v2 max":             cgroupFiles("0::/\n", map[string]string{"sys/fs/cgroup/memory.max": "max\n"}),
		"v1 unlimited":       cgroupFiles("4:memory:/\n", map[string]string{"sys/fs/cgroup/memory/memory.limit_in_bytes": "9223372036854771712\n"}),
		"missing membership": cgroupFiles("", map[string]string{"sys/fs/cgroup/memory.max": "1073741824"}),
		"malformed limit":    cgroupFiles("0::/\n", map[string]string{"sys/fs/cgroup/memory.max": "lots"}),
		"zero limit":         cgroupFiles("0::/\n", map[string]string{"sys/fs/cgroup/memory.max": "0"}),
		"no memory files":    cgroupFiles("0::/\n", nil),
		"no controller":      cgroupFiles("3:cpu:/\n", map[string]string{"sys/fs/cgroup/memory/memory.limit_in_bytes": "1073741824"}),
	} {
		if limit, source := memoryLimit(func(string) string { return "" }, files, 0.4); limit != 0 || source != "" {
			t.Fatalf("%s: limit = %d from %q", name, limit, source)
		}
	}
}

func TestExplicitGOMEMLIMITWinsAndIsReported(t *testing.T) {
	previous := debug.SetMemoryLimit(-1)
	t.Cleanup(func() { debug.SetMemoryLimit(previous) })
	var logs bytes.Buffer
	logger := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&logs, nil)))
	t.Cleanup(func() { slog.SetDefault(logger) })
	files := cgroupFiles("0::/\n", map[string]string{"sys/fs/cgroup/memory.max": "1073741824"})
	applyMemoryLimit(func(name string) string { return map[string]string{"GOMEMLIMIT": "900MiB"}[name] }, files, 0.4)
	if debug.SetMemoryLimit(-1) != previous {
		t.Fatalf("explicit GOMEMLIMIT was overridden: %d", debug.SetMemoryLimit(-1))
	}
	applyMemoryLimit(func(string) string { return "" }, files, 0.4)
	if debug.SetMemoryLimit(-1) != 429496729 {
		t.Fatalf("cgroup memory limit = %d", debug.SetMemoryLimit(-1))
	}
	output := logs.String()
	if strings.Count(output, `"msg":"Go memory limit configured"`) != 2 || !strings.Contains(output, `"source":"GOMEMLIMIT"`) || !strings.Contains(output, `"source":"cgroup","limit_bytes":429496729`) || !strings.Contains(output, `"level":"INFO"`) {
		t.Fatalf("memory limit logs = %s", output)
	}
}
