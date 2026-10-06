//go:build q47proof

package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"
)

func die(code string) {
	fmt.Fprintln(os.Stderr, "q47 fixture: "+code)
	os.Exit(2)
}
func openSite(site string) (*os.Root, error) {
	temporary := os.Getenv("RUNNER_TEMP")
	if len(site) > 4096 || !filepath.IsAbs(site) || site != filepath.Clean(site) ||
		!filepath.IsAbs(temporary) || temporary == "/" {
		return nil, errors.New("site boundary")
	}
	relative, err := filepath.Rel(temporary, site)
	if err != nil || relative == "." || !filepath.IsLocal(relative) {
		return nil, errors.New("site containment")
	}
	parent, err := os.OpenRoot(temporary)
	if err != nil {
		return nil, err
	}
	root, openErr := parent.OpenRoot(relative)
	return root, errors.Join(openErr, parent.Close())
}
func readPublished(root *os.Root, name string) ([]byte, error) {
	file, err := root.Open(name)
	if err != nil {
		return nil, err
	}
	data, readErr := io.ReadAll(io.LimitReader(file, 16385))
	if errors.Join(readErr, file.Close()) != nil || len(data) < 32 || len(data) > 16384 {
		return nil, errors.New("template unavailable")
	}
	return data, nil
}
func newPeer(root *os.Root) (*peer, error) {
	p := &peer{files: http.FileServerFS(root.FS()), templates: map[string][]byte{}}
	for app := range apps {
		data, err := readPublished(root, "assets/install/"+app+".yaml")
		if err != nil {
			return nil, errors.New("template bound")
		}
		p.templates[app] = data
	}
	p.reset("valid", "player")
	return p, nil
}

func main() {
	root, err := openSite(os.Getenv("KINOSAIL_Q47_SITE"))
	if err != nil {
		die("site")
	}
	p, err := newPeer(root)
	if err != nil {
		die("assets")
	}
	listener, err := net.ListenTCP("tcp4", &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 41847})
	if err != nil {
		die("listen")
	}
	server := &http.Server{Handler: http.HandlerFunc(p.serve), ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout: 5 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 5 * time.Second,
		MaxHeaderBytes: 8192, ErrorLog: log.New(io.Discard, "", 0)}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	go func() {
		if err := server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			die("serve")
		}
	}()
	fmt.Println("{\"schemaVersion\":1,\"kind\":\"q47-ready\",\"ready\":true}")
	lifetime, stopLifetime := context.WithTimeout(ctx, 240*time.Second)
	defer stopLifetime()
	<-lifetime.Done()
	shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdown); err != nil {
		if err := server.Close(); err != nil {
			die("shutdown")
		}
	}
	if err := root.Close(); err != nil {
		die("root")
	}
	fmt.Println("{\"schemaVersion\":1,\"kind\":\"q47-stopped\",\"stopped\":true}")
}
