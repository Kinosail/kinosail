package main

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/MikeO7/kinosail-player/internal/server"
)

func main() {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		panic(err)
	}
	handler := server.New(server.Config{MediaDir: os.Args[1], DataDir: os.Args[2], FFprobe: os.Args[3], FFmpeg: os.Args[4]})
	fmt.Println("http://" + listener.Addr().String())
	service := &http.Server{Handler: handler, ReadHeaderTimeout: 5 * time.Second}
	if err := service.Serve(listener); err != nil {
		panic(err)
	}
}
