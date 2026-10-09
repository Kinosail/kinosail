package server

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/MikeO7/kinosail/packages/library"
	"github.com/MikeO7/kinosail/packages/servertest/mp4fixture"
)

func copiedRecoveryEnclosingFixture(t *testing.T) (*hlsManager, library.Item, hlsRecipe, string, string) {
	t.Helper()
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
	return manager, item, recipe, directory, initialization
}

func copiedRecoveryEnclosingMaster(t *testing.T, directory string) func(bool) {
	t.Helper()
	master := filepath.Join(directory, "index.m3u8")
	before, err := os.ReadFile(master)
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(master)
	if err != nil {
		t.Fatal(err)
	}
	return func(sameIdentity bool) {
		t.Helper()
		after, err := os.ReadFile(master)
		current, statErr := os.Stat(master)
		if err != nil || statErr != nil || !bytes.Equal(before, after) || sameIdentity && !sameCopiedHLSFile(info, current) {
			t.Error("enclosing zero refill replaced the committed master bytes or inode")
		}
	}
}

func copiedRecoveryRunEnclosing(ctx context.Context, manager *hlsManager, item library.Item, recipe hlsRecipe, directory string) error {
	options, err := manager.hlsSettings(item, recipe)
	if err != nil {
		return err
	}
	return manager.encodeVariants(ctx, item, directory, options, recipe, 0)
}

// Public media cannot schedule early selection or replacement-generation races.
func TestCopiedRecoveryEnclosingRefillPreservesMaster(t *testing.T) {
	manager, item, recipe, directory, initialization := copiedRecoveryEnclosingFixture(t)
	check := copiedRecoveryEnclosingMaster(t, directory)
	copiedRecoveryEncoderOutput(t, manager, "", initialization, "first fragment", copiedRecoveryManifest)
	if err := copiedRecoveryRunEnclosing(t.Context(), manager, item, recipe, directory); err != nil {
		t.Error("enclosing publisher rejected a valid retained zero refill")
	}
	check(true)
}

func TestCopiedRecoveryEnclosingOrdinaryPublication(t *testing.T) {
	manager, item, recipe, directory, initialization := copiedRecoveryEnclosingFixture(t)
	for _, name := range []string{".copy-timeline", ".copy-clock", "index.m3u8"} {
		if err := os.Remove(filepath.Join(directory, name)); err != nil {
			t.Fatal(err)
		}
	}
	copiedRecoveryEncoderOutput(t, manager, "", initialization, "first fragment", copiedRecoveryManifest)
	if err := copiedRecoveryRunEnclosing(t.Context(), manager, item, recipe, directory); err != nil {
		t.Fatalf("ordinary cold publication failed: %v", err)
	}
	master, err := os.ReadFile(filepath.Join(directory, "index.m3u8"))
	if err != nil || !bytes.Contains(master, []byte("1080p/index.m3u8")) {
		t.Fatal("ordinary cold worker failed to publish its master")
	}
}

func TestCopiedRecoveryEnclosingEarlyFailurePreservesMaster(t *testing.T) {
	for _, admission := range []bool{false, true} {
		t.Run(strconv.FormatBool(admission), func(t *testing.T) {
			manager, item, recipe, directory, initialization := copiedRecoveryEnclosingFixture(t)
			check := copiedRecoveryEnclosingMaster(t, directory)
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			if admission {
				cancel()
			} else {
				writeHLSLoadingFile(t, filepath.Join(directory, ".copy-clock"), "{}")
			}
			marker := filepath.Join(t.TempDir(), "launched")
			copiedRecoveryEncoderOutput(t, manager, "printf started > "+copiedRecoveryQuote(marker), initialization, "first fragment", copiedRecoveryManifest)
			if err := copiedRecoveryRunEnclosing(ctx, manager, item, recipe, directory); err == nil {
				t.Fatal("early admission/selection failure reported success")
			}
			if _, err := os.Lstat(marker); !os.IsNotExist(err) {
				t.Fatal("early failure launched a codec process")
			}
			check(true)
		})
	}
}

func TestCopiedRecoveryEnclosingCancellationJoins(t *testing.T) {
	manager, item, recipe, directory, initialization := copiedRecoveryEnclosingFixture(t)
	check := copiedRecoveryEnclosingMaster(t, directory)
	marker := filepath.Join(t.TempDir(), "pid")
	copiedRecoveryEncoderOutput(t, manager, "printf '%s' $$ > "+copiedRecoveryQuote(marker)+"\nexec sleep 30", initialization, "first fragment", copiedRecoveryManifest)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- copiedRecoveryRunEnclosing(ctx, manager, item, recipe, directory) }()
	var pid []byte
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		pid, _ = os.ReadFile(marker)
		if len(pid) > 0 {
			break
		}
		time.Sleep(time.Millisecond)
	}
	cancel()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("canceled enclosing worker reported success")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("canceled enclosing worker did not settle")
	}
	copiedRecoveryAssertStopped(t, pid)
	check(true)
	if _, err := os.Lstat(filepath.Join(directory, "1080p/segment-00000.m4s")); !os.IsNotExist(err) {
		t.Fatal("canceled enclosing worker published zero")
	}
}

func TestCopiedRecoveryEnclosingReplacementGeneration(t *testing.T) {
	manager, item, recipe, directory, initialization := copiedRecoveryEnclosingFixture(t)
	check := copiedRecoveryEnclosingMaster(t, directory)
	marker := filepath.Join(t.TempDir(), "replacement-master-inode")
	action := "mv " + copiedRecoveryQuote(directory) + " " + copiedRecoveryQuote(directory+"-retired") + "\ncp -R " + copiedRecoveryQuote(directory+"-retired") + " " + copiedRecoveryQuote(directory)
	action += "\nls -i " + copiedRecoveryQuote(filepath.Join(directory, "index.m3u8")) + " | awk '{print $1}' > " + copiedRecoveryQuote(marker)
	copiedRecoveryEncoderOutput(t, manager, action, initialization, "first fragment", copiedRecoveryManifest)
	if err := copiedRecoveryRunEnclosing(t.Context(), manager, item, recipe, directory); err == nil {
		t.Fatal("enclosing worker accepted replacement generation")
	}
	check(false)
	info, statErr := os.Stat(filepath.Join(directory, "index.m3u8"))
	inode, markerErr := os.ReadFile(marker)
	if statErr != nil || markerErr != nil || string(bytes.TrimSpace(inode)) != strconv.FormatUint(info.Sys().(*syscall.Stat_t).Ino, 10) {
		t.Fatal("enclosing worker replaced the new generation's master inode")
	}
	if _, err := os.Lstat(filepath.Join(directory, "1080p/segment-00000.m4s")); !os.IsNotExist(err) {
		t.Fatal("enclosing worker wrote zero into replacement generation")
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
			_, _, _, output, err := manager.prepareCopiedHLSOutput(t.Context(), directory, "360p", string(policy), recipe.mode, 0)
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
