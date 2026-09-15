package mediaprobe

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/MikeO7/kinosail/packages/library"
	"github.com/MikeO7/kinosail/packages/playback"
	"github.com/MikeO7/kinosail/packages/privatefile"
)

const embeddedSubtitleLimit = 16 << 20

var (
	ErrEmbeddedSubtitleStream = errors.New("embedded subtitle stream is unsupported")
	errEmbeddedSubtitleOutput = errors.New("subtitle output is empty or too large")
)

// EmbeddedOptions supplies installation tools and optional presentation mapping.
type EmbeddedOptions struct {
	FFmpeg     string
	CacheDir   string
	Enrichment Enrichment
	Timeline   *playback.Timeline
}

// EmbeddedSubtitle is either a secure cache file or in-memory WebVTT data.
type EmbeddedSubtitle struct {
	Path   string
	Data   []byte
	Mapped bool
}

// EmbeddedHTTPAdapter keeps app-owned visibility and localization at the HTTP seam.
type EmbeddedHTTPAdapter struct {
	Item    func(*http.Request, string) (library.Item, bool)
	Options func() EmbeddedOptions
	Error   func(http.ResponseWriter, *http.Request, string, int)
}

// EmbeddedHandler serves Player's canonical embedded subtitle route.
func (probe *Probe) EmbeddedHandler(adapter EmbeddedHTTPAdapter, policy playback.HLSRecipePolicy) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		item, found := adapter.Item(request, request.PathValue("id"))
		stream, err := strconv.Atoi(request.PathValue("stream"))
		if !found || err != nil {
			adapter.Error(writer, request, "not found", http.StatusNotFound)
			return
		}
		if message, status := probe.ServeEmbedded(writer, request, item, stream, policy, adapter.Options()); status != 0 {
			adapter.Error(writer, request, message, status)
		}
	}
}

// ServeEmbedded writes one validated playback subtitle and returns a localized error input.
func (probe *Probe) ServeEmbedded(writer http.ResponseWriter, request *http.Request, item library.Item, stream int, policy playback.HLSRecipePolicy, options EmbeddedOptions) (string, int) {
	started := time.Now()
	subtitle, err := probe.EmbeddedPlayback(request.Context(), item, stream, request.URL.Query().Get("playbackToken"), policy, options)
	if errors.Is(err, ErrEmbeddedSubtitleStream) {
		return "not found", http.StatusNotFound
	}
	if err != nil {
		slog.WarnContext(request.Context(), "embedded subtitle extraction failed", "stream", stream, "duration_ms", time.Since(started).Milliseconds(), "canceled", request.Context().Err() != nil, "error", playback.HLSDiagnostic(err, item.Path, options.CacheDir))
		return "embedded subtitles are unavailable", http.StatusServiceUnavailable
	}
	subtitle.ServeHTTP(writer, request, item.Added)
	return "", 0
}

// ServeHTTP writes the extracted subtitle with Player's canonical HTTP behavior.
func (subtitle EmbeddedSubtitle) ServeHTTP(writer http.ResponseWriter, request *http.Request, modified time.Time) {
	writer.Header().Set("Content-Type", "text/vtt; charset=utf-8")
	if subtitle.Mapped {
		_, _ = writer.Write(subtitle.Data)
		return
	}
	if subtitle.Path != "" {
		http.ServeFile(writer, request, subtitle.Path)
		return
	}
	http.ServeContent(writer, request, "subtitle.vtt", modified, bytes.NewReader(subtitle.Data))
}

// EmbeddedPlayback validates a playback token before it extracts a subtitle.
func (probe *Probe) EmbeddedPlayback(ctx context.Context, item library.Item, stream int, token string, policy playback.HLSRecipePolicy, options EmbeddedOptions) (EmbeddedSubtitle, error) {
	timeline, err := playback.TimelineFromPlaybackToken(token, policy)
	if err != nil {
		return EmbeddedSubtitle{}, ErrEmbeddedSubtitleStream
	}
	options.Timeline = &timeline
	return probe.Embedded(ctx, item, stream, options)
}

// Embedded validates, extracts, caches, and optionally maps one text subtitle stream.
func (probe *Probe) Embedded(ctx context.Context, item library.Item, stream int, options EmbeddedOptions) (EmbeddedSubtitle, error) {
	if stream < 0 || stream > 65535 {
		return EmbeddedSubtitle{}, ErrEmbeddedSubtitleStream
	}
	probe.ConfigureCache(options.CacheDir)
	if !textSubtitle(probe.Facts(ctx, item), stream) {
		return EmbeddedSubtitle{}, ErrEmbeddedSubtitleStream
	}
	path := embeddedSubtitlePath(options.CacheDir, item, stream)
	if data, err := readEmbeddedSubtitle(path); err == nil {
		return presentEmbeddedSubtitle(path, data, options.Timeline), nil
	}
	data, err := probe.embeddedData(ctx, item, stream, options, path)
	if err != nil {
		return EmbeddedSubtitle{}, err
	}
	return presentEmbeddedSubtitle(path, data, options.Timeline), nil
}

func textSubtitle(result Result, stream int) bool {
	for _, subtitle := range result.SubtitleFacts {
		if subtitle.SourceIndex == stream && subtitle.Text {
			return true
		}
	}
	return false
}

func embeddedSubtitlePath(cacheDir string, item library.Item, stream int) string {
	if cacheDir == "" || item.ID == "" || len(item.ID) > 256 || filepath.Base(item.ID) != item.ID {
		return ""
	}
	version := fmt.Sprintf("%x", sha256.Sum256([]byte(playback.SourceVersion(item.Path))))[:16]
	return filepath.Join(cacheDir, "embedded-subtitles", item.ID, version, strconv.Itoa(stream)+".vtt")
}

func readEmbeddedSubtitle(path string) ([]byte, error) {
	if path == "" {
		return nil, errors.New("embedded subtitle cache is unavailable")
	}
	data, err := privatefile.Read(path, embeddedSubtitleLimit)
	if err != nil || len(data) == 0 {
		return nil, errors.New("embedded subtitle cache is invalid")
	}
	return data, nil
}

func presentEmbeddedSubtitle(path string, data []byte, timeline *playback.Timeline) EmbeddedSubtitle {
	if timeline != nil && len(timeline.Omitted) > 0 {
		return EmbeddedSubtitle{Data: playback.MapWebVTT(data, *timeline), Mapped: true}
	}
	if path != "" {
		return EmbeddedSubtitle{Path: path}
	}
	return EmbeddedSubtitle{Data: data}
}

func extractEmbeddedSubtitle(ctx context.Context, ffmpeg, media string, stream int) ([]byte, error) {
	if ffmpeg == "" || len(ffmpeg) > 4096 || strings.ContainsRune(ffmpeg, '\x00') {
		return nil, errors.New("invalid FFmpeg executable")
	}
	output := boundedOutput{remaining: embeddedSubtitleLimit + 1}
	//nolint:gosec // The executable is installation config. The media path is scanned content. The stream index is probe-validated.
	command := exec.CommandContext(ctx, ffmpeg, "-v", "error", "-i", media, "-map", "0:"+strconv.Itoa(stream), "-f", "webvtt", "pipe:1")
	command.Stdout = &output
	err := command.Run()
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	if output.overflow || output.Len() > embeddedSubtitleLimit || err == nil && output.Len() == 0 {
		return nil, errEmbeddedSubtitleOutput
	}
	if err != nil {
		return nil, err
	}
	return output.Bytes(), nil
}
