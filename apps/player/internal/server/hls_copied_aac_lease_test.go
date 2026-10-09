package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"
)

type copiedAACSlowWriter struct {
	*httptest.ResponseRecorder
	entered, release chan struct{}
}

func (writer *copiedAACSlowWriter) Write(data []byte) (int,error) {
	select { case writer.entered<-struct{}{}: default: }
	<-writer.release
	return writer.ResponseRecorder.Write(data)
}

// Gap: genuine HTTP cannot reliably hold exactly after metadata admission.
func TestCopiedAACSlowNetworkDoesNotRetainMetadataLease(t *testing.T) {
	if runtime.GOOS!="linux" { t.Skip("qualified retained-source producer is Linux-only") }
	manager,directory,held:=copiedAACGenerationFixture(t)
	item,recipe:=held.item,held.recipe
	held.close()
	writer:=&copiedAACSlowWriter{ResponseRecorder:httptest.NewRecorder(),entered:make(chan struct{},1),release:make(chan struct{})}
	var releaseOnce sync.Once
	releaseDelivery:=func(){releaseOnce.Do(func(){close(writer.release)})}
	defer releaseDelivery()
	request:=httptest.NewRequestWithContext(t.Context(),http.MethodGet,"/init.mp4",nil)
	done:=make(chan bool,1)
	go func(){done<-manager.serveCopiedAACFile(writer,request,item,recipe,"360p/init.mp4",filepath.Base(directory))}()
	select {
	case <-writer.entered:
	case <-time.After(3*time.Second): t.Fatal("controlled asset did not reach retained-byte delivery")
	}
	ctx,cancel:=context.WithTimeout(t.Context(),200*time.Millisecond)
	defer cancel()
	_,release,err:=manager.copiedHLSMetadataAdmission(ctx)
	if err!=nil { t.Fatal("slow network retained the metadata gate after proof completed") }
	release()
	// Release delivery before returning and join the caller using its owned channel.
	releaseDelivery()
	select {
	case handled:=<-done:
		if !handled||writer.Code!=http.StatusOK||writer.Body.Len()==0 { t.Fatal("retained asset delivery failed after independent admission") }
	case <-time.After(time.Second): t.Fatal("controlled asset delivery did not join")
	}
}
