package appcli

import (
	"context"
	"io"
	"net"
	"net/http"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/MikeO7/kinosail/packages/remoteaccess"
	"github.com/MikeO7/kinosail/packages/servertransport"
	"github.com/MikeO7/kinosail/packages/trustedhttps"
)

func TestRunStopsWhenTheLauncherClosesItsPipe(t *testing.T) { //nolint:paralleltest // The test replaces the process-wide shutdown request.
	previous := processShutdown
	processShutdown = &shutdownRequest{done: make(chan struct{})}
	t.Cleanup(func() { processShutdown = previous })
	reader, writer := io.Pipe()
	StopWhenClosed(reader)
	listener, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	server := &http.Server{Handler: http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = writer.Write([]byte("running"))
	}), ReadHeaderTimeout: time.Second}
	serving := make(chan string, 1)
	result := make(chan int, 1)
	go func() {
		result <- run(testSettings{"remote.mode": "off"}, []os.Signal{os.Interrupt},
			func(context.Context, *sync.WaitGroup, *remoteaccess.Manager, *trustedhttps.Manager) *http.Server {
				return server
			},
			func(handler http.Handler) http.Handler { return handler },
			func(Settings) (*remoteaccess.Manager, error) { return nil, nil },
			func(Settings) (*trustedhttps.Manager, error) { return nil, nil },
			func(got *http.Server, _ servertransport.TLSConfig) error {
				serving <- listener.Addr().String()
				return got.Serve(listener)
			})
	}()
	request, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "http://"+<-serving, nil)
	if err != nil {
		t.Fatal(err)
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(response.Body)
	_ = response.Body.Close()
	if string(body) != "running" {
		t.Fatalf("server answered %q before the launcher closed", body)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	select {
	case code := <-result:
		if code != 0 {
			t.Fatalf("server stopped with code %d after the launcher closed", code)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("server kept running after the launcher closed its pipe")
	}
	stopped := make(chan struct{})
	go func() {
		RequestShutdown()
		RequestShutdown()
		close(stopped)
	}()
	select {
	case <-stopped:
	case <-time.After(time.Second):
		t.Fatal("a repeated shutdown request blocked its caller")
	}
}
