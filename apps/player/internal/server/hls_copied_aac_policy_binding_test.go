package server

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail/packages/library"
	"github.com/MikeO7/kinosail/packages/playback"
)

// Gap: generated cache fixtures write metadata directly; this is the actual
// shared publication API used to bind the selected producer's policy.
func TestCopiedAACPolicyBindsSharedSourcePublication(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("selected source inode policy is Linux-only")
	}
	item, info, base, options, directory, binding := copiedAACSharedPublication(t)
	bindingPath := filepath.Join(directory, ".source")
	master := filepath.Join(directory, "index.m3u8")
	writeHLSLoadingFile(t, master, "#EXTM3U\n#KINOSAIL-TRANSCODER:"+options.Cache+"\n#EXT-X-STREAM-INF:BANDWIDTH=1\n360p/index.m3u8\n")
	if !playback.MasterFresh(master, item.Path, options.Cache) || playback.MasterFresh(master, item.Path, base.Cache) {
		t.Fatal("master lost exact full-policy identity")
	}
	copiedAACMissingBindingFreshness(t, master, bindingPath, item.Path, options.Cache)
	if err := playback.BindHLSSource(directory, item.Path, base.Cache); err == nil {
		t.Fatal("different full binding replaced Version2 policy")
	}
	after, err := os.ReadFile(bindingPath)
	if err != nil || string(after) != string(binding) {
		t.Fatal("rejected binding changed preserved metadata")
	}
	copiedAACEqualStatReplacement(t, item.Path, info)
	if err := playback.ValidateHLSSource(item.Path, options.Cache); !errors.Is(err, playback.ErrHLSSourceChanged) {
		t.Fatal("equal-stat inode replacement retained selected source identity")
	}
	if playback.MasterFresh(master, item.Path, options.Cache) {
		t.Fatal("equal-stat inode replacement retained shared cache freshness")
	}
}

func TestCopiedAACMalformedPolicySuffixCannotBecomeLegacy(t *testing.T) {
	manager, item, _, _ := hlsLoadingFixture(t)
	base, err := manager.baseHLSSettings(item, hlsRecipe{mode: "remux"})
	if err != nil {
		t.Fatal(err)
	}
	for _, suffix := range []string{
		":copied-source=1.2:copied-aac=3",
		":copied-source=ABC.2:copied-aac=2",
		":copied-source=01.2:copied-aac=2",
		":copied-source=1.0:copied-aac=2",
		":copied-source=1:copied-aac=2",
		":copied-source=fffffffffffffffff.2:copied-aac=2",
		":copied-source=1.2:copied-aac=2:extra",
		":copied-aac=2:copied-source=1.2",
		":copied-source=1.2:copied-aac=2\r\n",
		":copied-source=" + strings.Repeat("1", 16<<10) + ".2:copied-aac=2",
		":copied-source=1.2:copied-aac=2:copied-source=1.2:copied-aac=2",
	} {
		directory := t.TempDir()
		if err := playback.BindHLSSource(directory, item.Path, base.Cache+suffix); err == nil {
			t.Fatal("malformed suffix reached source publication")
		}
		if _, err := os.Stat(filepath.Join(directory, ".source")); !os.IsNotExist(err) {
			t.Fatal("malformed suffix changed source metadata")
		}
		if err := playback.ValidateHLSSource(item.Path, base.Cache+suffix); err == nil {
			t.Fatal("noncanonical or unknown AAC suffix bypassed shared validation")
		}
	}
	if runtime.GOOS != "linux" {
		return
	}
	info, err := os.Stat(item.Path)
	if err != nil {
		t.Fatal(err)
	}
	token, err := copiedAACSourceToken(info)
	if err != nil {
		t.Fatal(err)
	}
	if err := playback.ValidateHLSSource(item.Path, strings.Replace(base.Cache, ":hls=15", ":hls=14", 1)+":copied-source="+token+":copied-aac=2"); err == nil {
		t.Fatal("Version2 suffix acquired an older producer policy")
	}
}

func TestCopiedAACSourcePolicyKeepsLegacyVersionsAndStampChecks(t *testing.T) {
	manager, item, _, _ := hlsLoadingFixture(t)
	base, err := manager.baseHLSSettings(item, hlsRecipe{mode: "remux"})
	if err != nil {
		t.Fatal(err)
	}
	for _, version := range []string{"15", "14", "13", "7"} {
		policy := strings.Replace(base.Cache, ":hls=15", ":hls="+version, 1)
		if err := playback.ValidateHLSSource(item.Path, policy); err != nil {
			t.Fatal("legacy source grammar changed")
		}
		if err := playback.BindHLSSource(t.TempDir(), item.Path, policy); err != nil {
			t.Fatal("legacy publication changed")
		}
	}
	content, err := os.ReadFile(item.Path)
	if err != nil {
		t.Fatal(err)
	}
	writeHLSLoadingFile(t, item.Path, string(content)+"changed")
	if err := playback.ValidateHLSSource(item.Path, base.Cache); !errors.Is(err, playback.ErrHLSSourceChanged) {
		t.Fatal("changed source stamp bypassed existing source validation")
	}
}

func copiedAACSharedPublication(t *testing.T) (library.Item, os.FileInfo, transcodeSettings, transcodeSettings, string, []byte) {
	t.Helper()
	manager, item, _, _ := hlsLoadingFixture(t)
	recipe := hlsRecipe{mode: "remux"}
	base, err := manager.baseHLSSettings(item, recipe)
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(item.Path)
	if err != nil {
		t.Fatal(err)
	}
	if err := manager.recordCopiedAACPolicy(hlsRecipeKey(item.ID, recipe), base.Cache, info, true, 1); err != nil {
		t.Fatal(err)
	}
	options, err := manager.hlsSettings(item, recipe)
	if err != nil {
		t.Fatal(err)
	}
	if err := playback.ValidateHLSSource(item.Path, options.Cache); err != nil {
		t.Fatal("selected AAC policy cannot pass the shared source validator")
	}
	directory := t.TempDir()
	if err := playback.BindHLSSource(directory, item.Path, options.Cache); err != nil {
		t.Fatal("selected AAC policy cannot bind actual cache publication")
	}
	bindingPath := filepath.Join(directory, ".source")
	binding, err := os.ReadFile(bindingPath)
	if err != nil || string(binding) != options.Cache {
		t.Fatal("publication lost the full selected policy")
	}
	return item, info, base, options, directory, binding
}

func copiedAACMissingBindingFreshness(t *testing.T, master, bindingPath, source, policy string) {
	t.Helper()
	if err := os.Rename(bindingPath, bindingPath+"-held"); err != nil {
		t.Fatal(err)
	}
	if playback.MasterFresh(master, source, policy) {
		t.Fatal("missing P2 source binding acquired legacy freshness")
	}
	if err := os.Rename(bindingPath+"-held", bindingPath); err != nil {
		t.Fatal(err)
	}
}
