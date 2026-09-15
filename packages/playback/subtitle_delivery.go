package playback

import (
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/MikeO7/kinosail/packages/library"
)

const maxSubtitleBytes = 4 << 20

type SubtitleDeliveryDependencies struct {
	ParseTimeline func(string) (Timeline, error)
	NotFound      func(http.ResponseWriter, *http.Request)
	Error         func(http.ResponseWriter, *http.Request, string, int)
}

type LibrarySubtitleDependencies struct {
	Lookup   func(*http.Request, string) (library.Item, bool)
	Safe     func(string) bool
	Delivery SubtitleDeliveryDependencies
}

// SubtitlePath selects one validated sidecar track.
func SubtitlePath(paths []string, value string) (string, bool) {
	if value == "" {
		value = "0"
	}
	track, err := strconv.Atoi(value)
	if err != nil || track < 0 || track >= len(paths) {
		return "", false
	}
	return paths[track], true
}

// ServeSubtitle delivers one safe app-selected sidecar as WebVTT.
func ServeSubtitle(writer http.ResponseWriter, request *http.Request, path string, timelines []Timeline, dependencies SubtitleDeliveryDependencies) { //nolint:cyclop // Validation, direct delivery, and conversion form one response boundary.
	if request == nil || dependencies.ParseTimeline == nil || dependencies.NotFound == nil || dependencies.Error == nil {
		http.Error(writer, "subtitle delivery is unavailable", http.StatusInternalServerError)
		return
	}
	if path == "" {
		dependencies.NotFound(writer, request)
		return
	}
	if len(timelines) == 0 && request.URL.Query().Get("playbackToken") != "" {
		timeline, err := dependencies.ParseTimeline(request.URL.Query().Get("playbackToken"))
		if err != nil {
			dependencies.NotFound(writer, request)
			return
		}
		timelines = append(timelines, timeline)
	}
	vtt := strings.EqualFold(filepath.Ext(path), ".vtt")
	if vtt && len(timelines) == 0 {
		setSubtitleHeaders(writer)
		//nolint:gosec // G703: the app validates that path belongs to scanned Library Content.
		http.ServeFile(writer, request, path)
		return
	}
	data, err := readSubtitle(path)
	if errors.Is(err, os.ErrNotExist) || errors.Is(err, os.ErrPermission) {
		dependencies.NotFound(writer, request)
		return
	}
	if err != nil {
		dependencies.Error(writer, request, "could not read subtitles", http.StatusInternalServerError)
		return
	}
	data = normalizeSubtitle(data, vtt)
	if len(timelines) > 0 {
		data = MapWebVTT(data, timelines[0])
	}
	setSubtitleHeaders(writer)
	//nolint:gosec // G705: text/vtt with nosniff is rendered only as a media subtitle track.
	_, _ = writer.Write(data)
}

// LibrarySubtitleHandler selects one safe track before starting delivery.
func LibrarySubtitleHandler(dependencies LibrarySubtitleDependencies) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		if request == nil || dependencies.Lookup == nil || dependencies.Safe == nil || dependencies.Delivery.NotFound == nil {
			http.Error(writer, "subtitle delivery is unavailable", http.StatusInternalServerError)
			return
		}
		item, found := dependencies.Lookup(request, request.PathValue("id"))
		path, valid := SubtitlePath(item.Subtitles, request.PathValue("track"))
		if !found || !valid || !dependencies.Safe(path) {
			dependencies.Delivery.NotFound(writer, request)
			return
		}
		ServeSubtitle(writer, request, path, nil, dependencies.Delivery)
	}
}

func readSubtitle(path string) ([]byte, error) {
	//nolint:gosec // G304: the app validates that path belongs to scanned Library Content.
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, maxSubtitleBytes+1))
	if err == nil && len(data) > maxSubtitleBytes {
		err = errors.New("subtitle is too large")
	}
	return data, err
}

func normalizeSubtitle(data []byte, vtt bool) []byte {
	lines := strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")
	for index, line := range lines {
		if strings.Contains(line, " --> ") {
			lines[index] = strings.ReplaceAll(line, ",", ".")
		}
	}
	data = []byte(strings.Join(lines, "\n"))
	if !vtt {
		data = append([]byte("WEBVTT\n\n"), data...)
	}
	return data
}

func setSubtitleHeaders(writer http.ResponseWriter) {
	writer.Header().Set("Content-Type", "text/vtt; charset=utf-8")
	writer.Header().Set("X-Content-Type-Options", "nosniff")
}
