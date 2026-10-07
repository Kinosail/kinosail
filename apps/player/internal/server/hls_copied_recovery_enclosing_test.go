package server

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail/packages/servertest/mp4fixture"
)

// Actual enclosing publication must preserve the already advertised master.
func TestCopiedRecoveryEnclosingRefillPreservesMaster(t *testing.T) { //nolint:cyclop,gocognit // Serial integration assertions and injected filesystem failure cases.
	manager, item, recipe, directory, policy, timeline := copiedRecoveryFixture(t)
	if err := os.Rename(filepath.Join(directory, "360p"), filepath.Join(directory, "1080p")); err != nil {
		t.Fatal(err)
	}
	initialization := string(mp4fixture.Initialization(1920, 1080, "h264", "aac", ""))
	writeHLSLoadingFile(t, filepath.Join(directory, "1080p/init.mp4"), initialization)
	master := filepath.Join(directory, "index.m3u8")
	writeHLSLoadingFile(t, master, "#EXTM3U\n#KINOSAIL-TRANSCODER:"+policy+"\n#EXT-X-STREAM-INF:BANDWIDTH=6128000\n1080p/index.m3u8\n")
	copiedRecoveryProbe(t, manager, "")
	if err := manager.bindCopiedHLSClock(t.Context(), item, recipe, directory, "1080p/index.m3u8", policy, timeline); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(directory, "1080p/segment-00000.m4s")); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(master)
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(master)
	if err != nil {
		t.Fatal(err)
	}
	copiedRecoveryEncoderOutput(t, manager, "", initialization, "first fragment", copiedRecoveryManifest)
	options, err := manager.hlsSettings(item, recipe)
	if err != nil {
		t.Fatal(err)
	}
	workerErr := manager.encodeVariants(t.Context(), item, directory, options, recipe, 0)
	after, err := os.ReadFile(master)
	current, statErr := os.Stat(master)
	if err != nil || statErr != nil || !bytes.Equal(before, after) || !sameCopiedHLSFile(info, current) {
		t.Error("enclosing zero refill replaced the committed master bytes or inode")
	}
	if workerErr != nil {
		t.Error("enclosing publisher rejected a valid retained zero refill")
	}
}

func TestCopiedRecoveryClockSnapshotRejectsDuplicateAndUnknownFields(t *testing.T) {
	for _, duplicate := range []bool{false, true} {
		t.Run(map[bool]string{false: "unknown", true: "duplicate"}[duplicate], func(t *testing.T) {
			manager, _, recipe, directory := copiedRecoveryRefill(t)
			policy, err := os.ReadFile(filepath.Join(directory, ".source"))
			if err != nil {
				t.Fatal(err)
			}
			_, _, output, err := manager.prepareCopiedHLSOutput(t.Context(), directory, "360p", string(policy), recipe.mode, 0)
			if err != nil {
				t.Fatal(err)
			}
			defer output.close()
			certificate := string(output.certificateData)
			if duplicate {
				certificate = strings.Replace(certificate, `"version":1`, `"version":1,"version":1`, 1)
			} else {
				certificate = strings.TrimSuffix(certificate, "}") + `,"extra":1}`
			}
			writeHLSLoadingFile(t, filepath.Join(directory, ".copy-clock"), certificate)
			if err := output.snapshot("360p"); err == nil {
				t.Fatal("later clock snapshot relaxed the unique closed JSON boundary")
			}
		})
	}
}
