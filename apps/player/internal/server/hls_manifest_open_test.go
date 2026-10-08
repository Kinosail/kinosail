package server

import (
	"bytes"
	"errors"
	"os"
	"testing"
	"time"
)

// This phase seam owns the exact inspected-path replacement before opening;
// HTTP overlap below is supplementary stress, not a deterministic schedule.
func TestHLSManifestAcceptsCompleteReplacementAfterInspection(t *testing.T) {
	root := hotManifestRoot(t, []byte(hotManifestOriginal))
	inspected, err := root.Lstat("index.m3u8")
	if err != nil {
		t.Fatal(err)
	}
	replacement := []byte(hotManifestOriginal + "#EXTINF:4,\nsegment-00001.m4s\n")
	if err := root.WriteFile("next.m3u8", replacement, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := root.Rename("next.m3u8", "index.m3u8"); err != nil {
		t.Fatal(err)
	}
	current, err := root.Lstat("index.m3u8")
	if err != nil || os.SameFile(inspected, current) {
		t.Fatal("publisher did not replace inspected pathname")
	}
	data, err := readHLSManifestAfterInspection(root, inspected)
	if err != nil || !bytes.Equal(data, replacement) {
		t.Fatal("complete replacement rejected before opening")
	}
	if sameCopiedHLSFile(inspected, current) {
		t.Fatal("strict copied-metadata counterfactual no longer rejects changed identity")
	}
}

func TestHLSManifestPreOpenReplacementRejectsUnsafeTargets(t *testing.T) {
	for _, damage := range []string{"missing", "symlink", "outside symlink", "directory", "empty", "oversized", "fifo"} {
		t.Run(damage, func(t *testing.T) { rejectUnsafeManifestReplacement(t, damage) })
	}
}

func rejectUnsafeManifestReplacement(t *testing.T, damage string) {
	t.Helper()
	root := hotManifestRoot(t, []byte(hotManifestOriginal))
	inspected, err := root.Lstat("index.m3u8")
	if err != nil {
		t.Fatal(err)
	}
	changeHotManifest(t, root, damage)
	data, err := boundedManifestReplacementRead(t, root, inspected, damage)
	if len(data) != 0 || !errors.Is(err, errCopiedHLSIndex) || hlsManifestFailureStage(err) == "not_manifest" {
		t.Fatal("unsafe replacement lost bounded failure")
	}
	protected, err := root.ReadFile("protected")
	if err != nil || string(protected) != "keep" {
		t.Fatal("reader changed protected data")
	}
}

func boundedManifestReplacementRead(t *testing.T, root *os.Root, inspected os.FileInfo, damage string) ([]byte, error) {
	t.Helper()
	type result struct {
		data []byte
		err  error
	}
	done := make(chan result, 1)
	go func() { data, err := readHLSManifestAfterInspection(root, inspected); done <- result{data, err} }()
	select {
	case value := <-done:
		return value.data, value.err
	case <-time.After(time.Second):
		if damage == "fifo" {
			releaseHotManifestFIFO(t, root)
		}
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Fatal("reader did not join after owned FIFO release")
		}
		t.Fatal("manifest open blocked")
		return nil, nil
	}
}

func TestHLSManifestFailureStagesKeepSentinelAndPrivateReasons(t *testing.T) {
	root := hotManifestRoot(t, []byte(hotManifestOriginal))
	if _, err := readHLSManifestAfterInspection(root, nil); !errors.Is(err, errCopiedHLSIndex) || hlsManifestFailureStage(err) != "path" {
		t.Fatal("invalid inspected input admitted")
	}
	file, before, err := copiedHLSOpenFile(root, "index.m3u8", maximumHLSManifestBytes)
	if err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	data, err := readHLSManifestFile(root, file, before)
	if len(data) != 0 || !errors.Is(err, errCopiedHLSIndex) || hlsManifestFailureStage(err) != "read" || err.Error() != errCopiedHLSIndex.Error() {
		t.Fatal("closed descriptor exposed arbitrary read error")
	}
}

func TestHLSManifestDescriptorAndPathStagesRejectMutation(t *testing.T) {
	for _, damage := range []string{"in-place append", "in-place truncate", "in-place timestamp", "symlink"} {
		t.Run(damage, func(t *testing.T) { rejectManifestDescriptorMutation(t, damage) })
	}
}

func rejectManifestDescriptorMutation(t *testing.T, damage string) {
	t.Helper()
	root := hotManifestRoot(t, []byte(hotManifestOriginal))
	file, before, err := copiedHLSOpenFile(root, "index.m3u8", maximumHLSManifestBytes)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	changeHotManifest(t, root, damage)
	data, err := readHLSManifestFile(root, file, before)
	want := "descriptor"
	if damage == "symlink" {
		want = "path"
	}
	if len(data) != 0 || !errors.Is(err, errCopiedHLSIndex) || hlsManifestFailureStage(err) != want {
		t.Fatal("unsafe descriptor/path mutation lost precise bounded stage")
	}
}
