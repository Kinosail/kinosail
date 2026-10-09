package server

import (
    "context"
    "crypto/sha256"
    "reflect"
    "net/http"
    "net/http/httptest"
    "os"
    "path/filepath"
    "runtime"
    "strings"
    "testing"
    "time"
)

// Gap: public HTTP cannot deterministically replace an opened generation between proof and rendering.
func TestCopiedAACPlaylistUsesRetainedGeneration(t *testing.T) {
    if runtime.GOOS != "linux" { t.Skip("qualified retained-source producer is Linux-only") }
    _,directory,held:=copiedAACGenerationFixture(t)
    defer held.close()
    for _,name:=range []string{"index.m3u8","360p/index.m3u8"} {
        data,err:=held.playlist(name,0,20,httptest.NewRequestWithContext(t.Context(),http.MethodGet,"/index.m3u8",nil))
        if err!=nil||!strings.HasPrefix(string(data),"#EXTM3U\n") { t.Fatal("valid retained playlist was not rendered") }
    }
    retired:=directory+"-playlist-retired"
    if err:=os.Rename(directory,retired);err!=nil { t.Fatal(err) }
    if err:=os.CopyFS(directory,os.DirFS(retired));err!=nil { t.Fatal(err) }
    if _,err:=held.playlist("index.m3u8",0,20,httptest.NewRequestWithContext(t.Context(),http.MethodGet,"/index.m3u8",nil));err==nil {
        t.Fatal("replacement canonical generation inherited retained playlist admission")
    }
    held.close()
}

func TestCopiedAACPlaylistRejectsMissingBindingWithoutMutation(t *testing.T) {
    if runtime.GOOS != "linux" { t.Skip("qualified retained-source producer is Linux-only") }
    manager,directory,held:=copiedAACGenerationFixture(t)
    item,recipe:=held.item,held.recipe
    held.close()
    before:=copiedAACPlaylistSnapshot(t,directory)
    if err:=os.Rename(filepath.Join(directory,".source"),filepath.Join(directory,".source-hidden"));err!=nil { t.Fatal(err) }
    missing:=copiedRecoveryPreserved(t,directory)
    for _,name:=range []string{"index.m3u8","360p/index.m3u8","720p/index.m3u8"} {
        writer:=httptest.NewRecorder()
        request:=httptest.NewRequestWithContext(t.Context(),http.MethodGet,"/"+name,nil)
        if !manager.serveCopiedAACPlaylist(writer,request,item,recipe,name,filepath.Base(directory),0,20)||writer.Code!=http.StatusNotFound {
            t.Fatalf("missing binding published %s",name)
        }
        if !reflect.DeepEqual(copiedAACPlaylistSnapshot(t,directory),missing) {t.Fatal("rejected playlist mutated cache")}
    }
    if err:=os.Rename(filepath.Join(directory,".source-hidden"),filepath.Join(directory,".source"));err!=nil { t.Fatal(err) }
    if !reflect.DeepEqual(copiedAACPlaylistSnapshot(t,directory),before) {t.Fatal("rejected playlist changed restored binding")}
}

// Gap: the public transport cannot hold one exact post-admission write to prove lease release.
func TestCopiedAACPlaylistSlowTransferReleasesMetadataLease(t *testing.T) {
    if runtime.GOOS != "linux" { t.Skip("qualified retained-source producer is Linux-only") }
    manager,directory,held:=copiedAACGenerationFixture(t)
    item,recipe:=held.item,held.recipe
    held.close()
    writer:=&copiedAACSlowWriter{ResponseRecorder:httptest.NewRecorder(),entered:make(chan struct{},1),release:make(chan struct{})}
    ctx,cancel:=context.WithCancel(t.Context())
    done:=make(chan bool,1)
    joined:=false
    released:=false
    release:=func(){if !released {close(writer.release);released=true}}
    t.Cleanup(func(){
        cancel()
        release()
        if !joined {select {case <-done: joined=true;case <-time.After(3*time.Second): t.Error("owned playlist caller did not join")}}
    })
    request:=httptest.NewRequestWithContext(ctx,http.MethodGet,"/index.m3u8",nil)
    go func(){done<-manager.serveCopiedAACPlaylist(writer,request,item,recipe,"index.m3u8",filepath.Base(directory),0,20)}()
    select {case <-writer.entered:case <-time.After(3*time.Second):t.Fatal("playlist did not reach retained-byte delivery")}
    admission,cancelAdmission:=context.WithTimeout(t.Context(),200*time.Millisecond)
    defer cancelAdmission()
    _,releaseAdmission,err:=manager.copiedHLSMetadataAdmission(admission)
    if err!=nil {t.Fatal("slow playlist retained metadata admission")}
    releaseAdmission()
    release()
    select {case handled:=<-done: joined=true;if !handled||writer.Code!=http.StatusOK||writer.Body.Len()==0 {t.Fatal("retained playlist delivery failed")};case <-time.After(time.Second):t.Fatal("owned playlist delivery did not join")}
}

func copiedAACPlaylistSnapshot(t *testing.T,directory string) map[string][32]byte {
    t.Helper()
    result:=map[string][32]byte{}
    for _,name:=range []string{".source",".copy-timeline",".copy-clock","index.m3u8","360p/index.m3u8","360p/init.mp4","360p/segment-00000.m4s","360p/segment-00001.m4s"} {
        data,err:=os.ReadFile(filepath.Join(directory,name))
        if os.IsNotExist(err) {continue}
        if err!=nil {t.Fatal(err)}
        result[name]=sha256.Sum256(data)
    }
    return result
}

func TestCopiedAACPlaylistRejectsUnboundMasterURISet(t *testing.T) {
    if runtime.GOOS!="linux" {t.Skip("qualified retained-source producer is Linux-only")}
    manager,directory,held:=copiedAACGenerationFixture(t)
    item,recipe,policy:=held.item,held.recipe,held.policy
    held.close()
    original,err:=os.ReadFile(filepath.Join(directory,"index.m3u8"))
    if err!=nil {t.Fatal(err)}
    for _,master:=range []string{string(original)+"../escape.m3u8\n",string(original)+"360p/index.m3u8\n",strings.Replace(string(original),policy,policy+"-wrong",1),string(original)+"720p/index.m3u8\n"} {
        writeHLSLoadingFile(t,filepath.Join(directory,"index.m3u8"),master)
        before:=copiedAACPlaylistSnapshot(t,directory)
        response:=httptest.NewRecorder()
        request:=httptest.NewRequestWithContext(t.Context(),http.MethodGet,"/index.m3u8",nil)
        if !manager.serveCopiedAACPlaylist(response,request,item,recipe,"index.m3u8",filepath.Base(directory),0,20)||response.Code!=http.StatusNotFound {t.Fatal("unbound master URI set was served")}
        if !reflect.DeepEqual(copiedAACPlaylistSnapshot(t,directory),before) {t.Fatal("rejected master changed cache")}
    }
}
