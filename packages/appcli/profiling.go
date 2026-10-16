package appcli

import (
	"context"
	"errors"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"net/http/pprof"
	"os"
	"path/filepath"
	"runtime/trace"
	"time"
)

const (
	profilingVariable  = "KINOSAIL_PROFILING"
	portableSocketPath = 103
)

func startProfiling(getenv func(string) string, dataDir string) func() {
	if getenv(profilingVariable) != "1" {
		return func() {}
	}
	listener, location, release, err := profilingListener(dataDir)
	if err != nil {
		slog.Warn("profiling socket unavailable", "operation", "profiling", "error", err)
		return func() {}
	}
	recorder := trace.NewFlightRecorder(trace.FlightRecorderConfig{MinAge: 30 * time.Second, MaxBytes: 16 << 20})
	if err := recorder.Start(); err != nil {
		slog.Warn("flight recorder unavailable", "operation", "profiling", "error", err)
	}
	server := &http.Server{Handler: profilingHandler(recorder), ReadHeaderTimeout: 5 * time.Second}
	go func() { _ = server.Serve(listener) }()
	slog.Info("profiling socket listening", "operation", "profiling", "socket", location)
	return func() {
		_ = server.Close()
		release()
		if recorder.Enabled() {
			recorder.Stop()
		}
	}
}

func profilingHandler(recorder *trace.FlightRecorder) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /debug/pprof/", pprof.Index)
	mux.HandleFunc("GET /debug/pprof/cmdline", pprof.Cmdline)
	mux.HandleFunc("GET /debug/pprof/profile", pprof.Profile)
	mux.HandleFunc("GET /debug/pprof/symbol", pprof.Symbol)
	mux.HandleFunc("GET /debug/pprof/trace", pprof.Trace)
	mux.HandleFunc("GET /debug/pprof/flight-recorder", func(writer http.ResponseWriter, _ *http.Request) {
		if !recorder.Enabled() {
			http.Error(writer, "flight recorder is not running", http.StatusServiceUnavailable)
			return
		}
		writer.Header().Set("Content-Type", "application/octet-stream")
		writer.Header().Set("Content-Disposition", `attachment; filename="kinosail-flight.trace"`)
		if _, err := recorder.WriteTo(writer); err != nil {
			slog.Warn("flight recorder snapshot failed", "operation", "profiling", "error", err)
		}
	})
	return mux
}

func profilingListener(dataDir string) (net.Listener, string, func(), error) {
	socket := filepath.Join(dataDir, "profiling", "pprof.sock")
	if len(socket) <= portableSocketPath {
		listener, err := listenPrivateSocket(socket)
		return listener, "profiling/pprof.sock", func() {}, err
	}
	directory, err := os.MkdirTemp("", "kinosail-profiling-")
	if err != nil {
		return nil, "", nil, err
	}
	socket = filepath.Join(directory, "pprof.sock")
	listener, err := listenPrivateSocket(socket)
	if err != nil {
		_ = os.Remove(directory)
		return nil, "", nil, err
	}
	slog.Warn("profiling socket path is too long for the data directory; using a private temporary directory", "operation", "profiling", "outcome", "fallback", "socket", socket)
	return listener, socket, func() { _ = os.Remove(directory) }, nil
}

func listenPrivateSocket(socket string) (net.Listener, error) {
	directory := filepath.Dir(socket)
	if err := os.Mkdir(directory, 0o700); err != nil && !errors.Is(err, fs.ErrExist) {
		return nil, err
	}
	if info, err := os.Lstat(directory); err != nil || !info.IsDir() {
		return nil, errors.New("profiling directory is not a directory")
	}
	if err := os.Chmod(directory, 0o700); err != nil { //nolint:gosec // Only the owner may traverse the profiling directory.
		return nil, err
	}
	if info, err := os.Lstat(socket); err == nil {
		if info.Mode().Type() != fs.ModeSocket {
			return nil, errors.New("profiling socket path is occupied")
		}
		if err := os.Remove(socket); err != nil {
			return nil, err
		}
	}
	listener, err := (&net.ListenConfig{}).Listen(context.Background(), "unix", socket)
	if err != nil {
		return nil, err
	}
	if err := os.Chmod(socket, 0o600); err != nil {
		_ = listener.Close()
		return nil, err
	}
	return listener, nil
}
