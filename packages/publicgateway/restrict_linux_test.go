//go:build linux && (amd64 || arm64)

package publicgateway

import (
	"context"
	"errors"
	"io"
	"net"
	"os"
	"os/exec"
	"runtime"
	"sync"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

func TestGatewayNetworkRestrictionAcrossAllThreads(t *testing.T) {
	if os.Getenv("KINOSAIL_TEST_GATEWAY_FILTER") != "1" {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestGatewayNetworkRestrictionAcrossAllThreads$")
		cmd.Env = append(os.Environ(), "KINOSAIL_TEST_GATEWAY_FILTER=1")
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("restricted child: %v %s", err, output)
		}
		return
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	client, err := net.Dial("tcp", listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	if err = restrictNetwork(); err != nil {
		t.Fatal(err)
	}
	// Already opened public listeners must still accept and serve after filtering.
	accepted, err := listener.Accept()
	if err != nil {
		t.Fatal(err)
	}
	defer accepted.Close()
	_ = accepted.SetDeadline(time.Now().Add(time.Second))
	_ = client.SetDeadline(time.Now().Add(time.Second))
	if _, err = accepted.Write([]byte("ready")); err != nil {
		t.Fatal(err)
	}
	message := make([]byte, 5)
	if _, err = io.ReadFull(client, message); err != nil || string(message) != "ready" {
		t.Fatal("restricted listener stopped serving")
	}
	var workers sync.WaitGroup
	for range 8 {
		workers.Go(func() {
			runtime.LockOSThread()
			defer runtime.UnlockOSThread()
			for _, family := range []int{unix.AF_INET, unix.AF_INET6, unix.AF_PACKET} {
				fd, err := unix.Socket(family, unix.SOCK_STREAM, 0)
				if fd >= 0 {
					_ = unix.Close(fd)
				}
				if !errors.Is(err, unix.EPERM) {
					t.Errorf("IP socket = %v", err)
				}
			}
			fd, err := unix.Socket(unix.AF_UNIX, unix.SOCK_STREAM, 0)
			if err != nil {
				t.Errorf("private socket = %v", err)
			} else {
				_ = unix.Close(fd)
			}
		})
	}
	workers.Wait()
	_, _, errno := unix.Syscall(unix.SYS_IO_URING_SETUP, 0, 0, 0)
	if errno != unix.EPERM {
		t.Fatalf("io_uring bypass = %v", errno)
	}
}
