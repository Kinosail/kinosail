package server

import (
	"os"
	"path/filepath"
	"testing"
)

// Public E2E cannot schedule corruption between pending copy and atomic Link.
func TestCopiedRecoveryPendingMutationCannotPublish(t *testing.T) { //nolint:cyclop,gocognit // Serial integration assertions and injected filesystem failure cases.
	for _, action := range []string{"contents", "replacement", "identical-replacement", "cancel"} {
		t.Run(action, func(t *testing.T) {
			manager, _, recipe, directory := copiedRecoveryRefill(t)
			check := copiedRecoveryPreserved(t, directory)
			policy, err := os.ReadFile(filepath.Join(directory, ".source"))
			if err != nil {
				t.Fatal(err)
			}
			_, _, output, err := manager.prepareCopiedHLSOutput(t.Context(), directory, "360p", string(policy), recipe.mode, 0)
			if err != nil {
				t.Fatal(err)
			}
			defer output.close()
			writeHLSLoadingFile(t, filepath.Join(output.directory, "segment-00000.m4s"), "first fragment")
			armed := true
			err = output.publishFirst(func() bool {
				if !armed {
					return true
				}
				armed = false
				pending := filepath.Join(directory, "360p", filepath.Base(output.directory)+".pending")
				switch action {
				case "cancel":
					output.cancel()
				case "contents":
					writeHLSLoadingFile(t, pending, "tampered after copy")
				default:
					data := "tampered replacement"
					if action == "identical-replacement" {
						data = "first fragment"
					}
					writeHLSLoadingFile(t, pending+"-other", data)
					if err := os.Rename(pending+"-other", pending); err != nil {
						t.Fatal(err)
					}
				}
				return true
			})
			if err == nil {
				t.Error("changed or canceled pending fragment was accepted")
			}
			if _, err := os.Lstat(filepath.Join(directory, "360p/segment-00000.m4s")); !os.IsNotExist(err) {
				t.Fatal("changed pending fragment was linked into the committed cache")
			}
			check()
		})
	}
}

func TestCopiedRecoveryAfterLinkFailurePreservesRaceWinner(t *testing.T) {
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
	writeHLSLoadingFile(t, filepath.Join(output.directory, "segment-00000.m4s"), "first fragment")
	first := filepath.Join(directory, "360p/segment-00000.m4s")
	calls := 0
	err = output.publishFirst(func() bool {
		calls++
		if calls == 1 {
			return true
		}
		writeHLSLoadingFile(t, first+"-winner", "replacement producer")
		if err := os.Rename(first+"-winner", first); err != nil {
			t.Fatal(err)
		}
		return false
	})
	if err == nil || calls != 2 {
		t.Fatal("post-link failure control never executed")
	}
	data, err := os.ReadFile(first)
	if err != nil || string(data) != "replacement producer" {
		t.Fatal("failed publisher removed or changed another producer's replacement")
	}
}

func TestCopiedRecoveryAfterLinkFailureRemovesOnlyOwnedPublication(t *testing.T) {
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
	writeHLSLoadingFile(t, filepath.Join(output.directory, "segment-00000.m4s"), "first fragment")
	calls := 0
	err = output.publishFirst(func() bool { calls++; return calls == 1 })
	if err == nil || calls != 2 {
		t.Fatal("post-link failure control never executed")
	}
	if _, err := os.Lstat(filepath.Join(directory, "360p/segment-00000.m4s")); !os.IsNotExist(err) {
		t.Fatal("failed publisher left its own rejected publication")
	}
}
