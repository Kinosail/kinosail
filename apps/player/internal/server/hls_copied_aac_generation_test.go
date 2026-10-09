package server

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"os"
	"runtime"
	"path/filepath"
	"os/exec"
	"time"
	"encoding/hex"
	"testing"
)

// This fixture supplies metadata only; it does not claim an actual source packet proof.
func copiedAACGenerationFixture(t *testing.T) (*hlsManager, string, *copiedAACGeneration) {
	t.Helper()
	manager, item, recipe, directory, base, timeline := copiedRecoveryFixture(t)
	copiedAACSourceRoots(manager,item)
	info, err := os.Stat(item.Path)
	if err != nil { t.Fatal(err) }
	if err := manager.recordCopiedAACPolicy(hlsRecipeKey(item.ID, recipe), base, info, true, 1); err != nil { t.Fatal(err) }
	options, err := manager.hlsSettings(item, recipe)
	if err != nil { t.Fatal(err) }
	initialization, first, facts := copiedAACGeneratedMetadata(t)
	writeHLSLoadingFile(t,filepath.Join(directory,"360p/init.mp4"),string(initialization))
	writeHLSLoadingFile(t,filepath.Join(directory,"360p/segment-00000.m4s"),string(first))
	clock := 0.0
	timeline.Clock, timeline.Policy = &clock, options.Cache
	timeline.AudioOrigin = &copiedHLSAudioOrigin{SourceTrack: 1, FirstHash: "SHA256:"+hex.EncodeToString(facts.FirstPacket[:]), FirstPTS:-facts.OriginalMediaTime, Physical:facts.OriginalMediaTime, Edit:facts.OriginalMediaTime}
	data, err := json.Marshal(timeline)
	if err != nil { t.Fatal(err) }
	certificate, err := json.Marshal(copiedHLSClockCertificate{Version:2,Rendition:"360p",Timeline:sha256.Sum256(data),Initialization:sha256.Sum256(initialization),First:sha256.Sum256(first)})
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
	held.close() // Release the old proof lease before a new independent admission.
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

func copiedAACGeneratedMetadata(t *testing.T) ([]byte, []byte, *copiedHLSPrivateAudioFacts) {
	t.Helper()
	if os.Getenv("KINOSAIL_COPIED_RECOVERY_MEDIA")!="1" { t.Skip("pinned hosted media supplies otherwise-valid V2 metadata") }
	ffmpeg,err:=exec.LookPath("ffmpeg")
	if err!=nil { t.Fatal("pinned metadata fixture codec unavailable") }
	ctx,cancel:=context.WithTimeout(t.Context(),30*time.Second)
	defer cancel()
	directory:=t.TempDir()
	command:=exec.CommandContext(ctx,ffmpeg,"-nostdin","-v","error","-f","lavfi","-i","testsrc2=s=320x180:r=24:d=4","-f","lavfi","-i","sine=frequency=440:sample_rate=48000:duration=4","-c:v","libx264","-threads","2","-preset","veryfast","-g","48","-keyint_min","48","-sc_threshold","0","-c:a","aac","-ac","2","-avoid_negative_ts","disabled","-f","hls","-hls_time","2","-hls_playlist_type","event","-hls_segment_type","fmp4","-hls_segment_options","movflags=+skip_sidx:avoid_negative_ts=disabled:use_editlist=1","-hls_flags","temp_file","-hls_fmp4_init_filename","init.mp4","-hls_segment_filename",filepath.Join(directory,"segment-%05d.m4s"),filepath.Join(directory,"index.m3u8")) //nolint:gosec // Fixed bounded generated metadata on the designated hosted runner.
	if err:=command.Run();err!=nil { t.Fatal("otherwise-valid V2 metadata generation failed") }
	initialization:=remainingNonKeyCollectorRead(t,filepath.Join(directory,"init.mp4"),2<<20)
	first:=remainingNonKeyCollectorRead(t,filepath.Join(directory,"segment-00000.m4s"),64<<20)
	facts,err:=parseCopiedHLSPrivateAudio(ctx,initialization,first)
	if err!=nil { t.Fatal("otherwise-valid AAC initialization/first metadata missing") }
	return initialization,first,facts
}
