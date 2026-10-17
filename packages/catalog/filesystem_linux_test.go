//go:build linux

package catalog

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestNetworkMountsBelowSelectsOnlyNestedNetworkSubtrees(t *testing.T) {
	real := t.TempDir()
	root := filepath.Join(t.TempDir(), "media")
	if err := os.Symlink(real, root); err != nil {
		t.Fatal(err)
	}
	escaped := strings.ReplaceAll(real, " ", `\040`)
	mountinfo := strings.Join([]string{
		"22 1 8:1 / / rw,relatime - ext4 /dev/sda1 rw",
		"30 22 0:40 / " + escaped + " rw - ext4 /dev/sdb1 rw",
		"31 30 0:41 /share " + escaped + `/NAS\040Share rw,relatime - nfs4 server:/share rw`,
		"32 31 0:42 / " + escaped + `/NAS\040Share/Deeper rw - nfs4 server:/deeper rw`,
		"33 30 0:43 / " + escaped + "/Local rw - ext4 /dev/sdc1 rw",
		"34 22 0:44 / " + escaped + "-other/NAS rw - nfs server:/other rw",
		"35 30 0:45 / " + escaped + "/Cloud rw - fuse.rclone remote: rw",
		"malformed",
	}, "\n")
	network := func(path string) bool {
		return path != real+"/Local" && path != "/" && path != real
	}
	mounts := networkMountsBelow(root, strings.NewReader(mountinfo), network)
	slices.Sort(mounts)
	want := []string{filepath.Join(root, "Cloud"), filepath.Join(root, "NAS Share")}
	if !slices.Equal(mounts, want) {
		t.Fatalf("nested network mounts = %#v, want %#v", mounts, want)
	}
	if mounts := networkMountsBelow(filepath.Join(root, "missing"), strings.NewReader(mountinfo), network); len(mounts) != 0 {
		t.Fatalf("missing root mounts = %#v", mounts)
	}
}
