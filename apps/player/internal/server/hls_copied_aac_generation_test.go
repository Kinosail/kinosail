package server

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"os"
	"runtime"
	"path/filepath"
	"strings"
	"testing"
)

// This fixture supplies metadata only; it does not claim an actual source packet proof.
func copiedAACGenerationFixture(t *testing.T) (*hlsManager, string, *copiedAACGeneration) {
	t.Helper()
	manager, item, recipe, directory, base, timeline := copiedRecoveryFixture(t)
	info, err := os.Stat(item.Path)
	if err != nil { t.Fatal(err) }
	if err := manager.recordCopiedAACPolicy(hlsRecipeKey(item.ID, recipe), base, info, true, 1); err != nil { t.Fatal(err) }
	options, err := manager.hlsSettings(item, recipe)
	if err != nil { t.Fatal(err) }
	clock := 0.0
	timeline.Clock, timeline.Policy = &clock, options.Cache
	timeline.AudioOrigin = &copiedHLSAudioOrigin{SourceTrack: 1, FirstHash: "SHA256:"+strings.Repeat("a",64)}
	data, err := json.Marshal(timeline)
	if err != nil { t.Fatal(err) }
	certificate, err := json.Marshal(copiedHLSClockCertificate{Version:2,Rendition:"360p",Timeline:sha256.Sum256(data),Initialization:sha256.Sum256([]byte("initialization")),First:sha256.Sum256([]byte("first fragment"))})
	if err != nil { t.Fatal(err) }
	writeHLSLoadingFile(t, filepath.Join(directory,".source"),options.Cache)
	writeHLSLoadingFile(t, filepath.Join(directory,".copy-timeline"),string(data))
	writeHLSLoadingFile(t, filepath.Join(directory,".copy-clock"),string(certificate))
	writeHLSLoadingFile(t, filepath.Join(directory,"index.m3u8"),"#EXTM3U\n#KINOSAIL-TRANSCODER:"+options.Cache+"\n#EXT-X-STREAM-INF:BANDWIDTH=1000000\n360p/index.m3u8\n")
	value, err := manager.openCopiedAACGeneration(t.Context(),item,recipe,directory)
	if err != nil { t.Fatal(err) }
	return manager,directory,value
}

func TestCopiedAACReadinessAndAssetsRetainGeneration(t *testing.T) {
	if runtime.GOOS != "linux" { t.Skip("qualified retained-source producer is Linux-only") }
	manager,directory,held := copiedAACGenerationFixture(t)
	defer held.close()
	if !held.current() { t.Fatal("initial retained generation was not current") }
	retired := directory+"-retired"
	if err := os.Rename(directory,retired); err != nil { t.Fatal(err) }
	if err := os.CopyFS(directory,os.DirFS(retired)); err != nil { t.Fatal(err) }
	if held.current() { t.Fatal("replacement canonical generation inherited retained admission") }
	root,err := manager.openCopiedAACGeneration(t.Context(),held.item,held.recipe,directory)
	if err != nil { t.Fatal("independent new generation could not be verified") }
	defer root.close()
	if !root.current() { t.Fatal("replacement generation did not acquire its own admission") }
}

func TestCopiedAACGenerationRejectsWrongKindAndSourceReplacement(t *testing.T) {
	if runtime.GOOS != "linux" { t.Skip("qualified retained-source producer is Linux-only") }
	manager,directory,held := copiedAACGenerationFixture(t)
	defer held.close()
	var certificate copiedHLSClockCertificate
	if json.Unmarshal(held.certificateData,&certificate)!=nil { t.Fatal("fixture certificate missing") }
	certificate.Version=1
	data,err := json.Marshal(certificate)
	if err!=nil { t.Fatal(err) }
	writeHLSLoadingFile(t,filepath.Join(directory,".copy-clock"),string(data))
	if _,err := manager.readCopiedHLSTimelineContext(t.Context(),directory,held.policy); err==nil { t.Fatal("Version1 clock certified a Version2 origin") }
	writeHLSLoadingFile(t,filepath.Join(directory,".copy-clock"),string(held.certificateData))
	before,err := os.Stat(held.item.Path)
	if err!=nil { t.Fatal(err) }
	content,err := os.ReadFile(held.item.Path)
	if err!=nil { t.Fatal(err) }
	replacement:=held.item.Path+"-replacement"
	writeHLSLoadingFile(t,replacement,string(content))
	if err:=os.Chtimes(replacement,before.ModTime(),before.ModTime()); err!=nil { t.Fatal(err) }
	if err:=os.Rename(replacement,held.item.Path); err!=nil { t.Fatal(err) }
	if held.current() { t.Fatal("same-stat source inode replacement retained admission") }
	if err:=manager.copiedAACCacheAsset(context.Background(),held.item,held.recipe,directory,"360p/init.mp4"); err==nil { t.Fatal("source replacement downgraded direct asset delivery") }
}
