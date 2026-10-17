package mediaprobe

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/MikeO7/kinosail/packages/library"
	"github.com/MikeO7/kinosail/packages/playback"
	"github.com/MikeO7/kinosail/packages/privatefile"
	"github.com/MikeO7/kinosail/packages/workload"
	"golang.org/x/text/language"
)

const (
	imageSubtitleFrameLimit  = 40000
	imageSubtitleStreamLimit = 8
)

var (
	imageSubtitleCodecs        = []string{"hdmv_pgs_subtitle", "dvd_subtitle", "dvb_subtitle", "xsub"}
	ErrImageSubtitleOCR        = errors.New("image subtitle text recognition is unavailable")
	errImageSubtitleFrames     = errors.New("image subtitle frames are invalid")
	errImageSubtitleUnreadable = errors.New("image subtitle has no readable text")
)

type ImageTextOptions struct {
	FFmpeg, Tesseract, CacheDir string
	Language, TessdataDir       string
}

func ImageSubtitle(codec string) bool { return slices.Contains(imageSubtitleCodecs, lower(codec)) }

func (probe *Probe) WithConvertedSubtitles(item library.Item, result Result, cacheDir string) Result {
	converted := false
	for _, track := range result.SubtitleFacts {
		if ImageSubtitle(track.Codec) && privatefile.Exists(embeddedSubtitlePath(cacheDir, item, track.SourceIndex), embeddedSubtitleLimit) {
			converted = true
		}
	}
	if !converted {
		return result
	}
	result.SubtitleFacts = slices.Clone(result.SubtitleFacts)
	for index, track := range result.SubtitleFacts {
		if ImageSubtitle(track.Codec) && privatefile.Exists(embeddedSubtitlePath(cacheDir, item, track.SourceIndex), embeddedSubtitleLimit) {
			result.SubtitleFacts[index].Text, result.SubtitleFacts[index].Converted = true, true
		}
	}
	return result
}

func (probe *Probe) ImageTextCandidates(ctx context.Context, item library.Item, result Result, options ImageTextOptions) []int {
	recognized, _ := probe.ocrLanguage(ctx, options)
	if recognized == "" {
		return nil
	}
	var streams []int
	for _, track := range result.SubtitleFacts {
		if len(streams) == imageSubtitleStreamLimit || !ImageSubtitle(track.Codec) || OCRLanguage(track.Language) != recognized || hasTextTwin(item, result, track.Language, track.Forced) {
			continue
		}
		path := embeddedSubtitlePath(options.CacheDir, item, track.SourceIndex)
		if path != "" && !privatefile.Exists(path, embeddedSubtitleLimit) && !privatefile.Exists(path+".unreadable", 64) {
			streams = append(streams, track.SourceIndex)
		}
	}
	return streams
}

func hasTextTwin(item library.Item, result Result, code string, forced bool) bool {
	for _, track := range result.SubtitleFacts {
		if track.Text && !track.Converted && track.Forced == forced && OCRLanguage(track.Language) == OCRLanguage(code) {
			return true
		}
	}
	for _, path := range item.Subtitles {
		sidecarForced := strings.Contains(strings.ToLower(filepath.Base(path)), ".forced.")
		if sidecarForced == forced && OCRLanguage(playback.SubtitleFileLanguage(path)) == OCRLanguage(code) {
			return true
		}
	}
	return false
}

func (probe *Probe) ConvertImageSubtitles(ctx context.Context, item library.Item, streams []int, options ImageTextOptions) error {
	recognized, data := probe.ocrLanguage(ctx, options)
	if recognized == "" || !validFFmpegExecutable(options.FFmpeg) || len(streams) == 0 || len(streams) > imageSubtitleStreamLimit {
		return ErrImageSubtitleOCR
	}
	directory := filepath.Dir(embeddedSubtitlePath(options.CacheDir, item, streams[0]))
	if directory == "." {
		return ErrImageSubtitleOCR
	}
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return err
	}
	workspace, err := os.MkdirTemp(directory, ".ocr-")
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(workspace) }()
	if err = extractImageSubtitleFrames(ctx, options.FFmpeg, item.Path, workspace, streams); err != nil {
		return err
	}
	results := make([][]byte, len(streams))
	errs := make([]error, len(streams))
	for position, stream := range streams {
		results[position], errs[position] = recognizeImageSubtitle(ctx, options.Tesseract, recognized, data, filepath.Join(workspace, strconv.Itoa(stream)))
		if ctx.Err() != nil {
			return ctx.Err()
		}
	}
	_ = os.RemoveAll(workspace)
	var failures []error
	for position, stream := range streams {
		path := embeddedSubtitlePath(options.CacheDir, item, stream)
		switch err := errs[position]; {
		case errors.Is(err, errImageSubtitleUnreadable) || errors.Is(err, errImageSubtitleFrames):
			_ = privatefile.WriteCache(path+".unreadable", []byte("1"))
			failures = append(failures, fmt.Errorf("stream %d: %w", stream, err))
		case err != nil:
			failures = append(failures, fmt.Errorf("stream %d: %w", stream, err))
		default:
			if err = privatefile.WriteCache(path, results[position]); err != nil {
				return err
			}
		}
	}
	return errors.Join(failures...)
}

func extractImageSubtitleFrames(ctx context.Context, ffmpeg, media, workspace string, streams []int) error {
	graph := make([]string, 0, len(streams))
	outputs := make([]string, 0, len(streams)*12)
	for _, stream := range streams {
		output := filepath.Join(workspace, strconv.Itoa(stream))
		if err := os.Mkdir(output, 0o700); err != nil {
			return err
		}
		label := "s" + strconv.Itoa(stream)
		graph = append(graph, "[0:"+strconv.Itoa(stream)+"]format=rgba["+label+"]")
		outputs = append(outputs, "-map", "["+label+"]", "-fps_mode", "passthrough", "-frame_pts", "1", "-enc_time_base", "1/1000", "-frames:v", strconv.Itoa(imageSubtitleFrameLimit), "-f", "image2", "file:"+filepath.Join(output, "%012d.png"))
	}
	arguments := append([]string{"-v", "error", "-nostdin", "-i", media, "-filter_complex", strings.Join(graph, ";")}, outputs...)
	command := exec.CommandContext(ctx, ffmpeg, arguments...)
	err := workload.RunContext(ctx, command)
	if ctx.Err() != nil {
		return ctx.Err()
	}
	return err
}

func (probe *Probe) ConvertedSubtitle(item library.Item, stream int, cacheDir string) ([]byte, error) {
	return readEmbeddedSubtitle(embeddedSubtitlePath(cacheDir, item, stream))
}

func ocrLanguageCode(code string) string {
	tag, err := language.Parse(strings.TrimSpace(code))
	if err != nil || tag == language.Und {
		return ""
	}
	base, _ := tag.Base()
	if name := base.ISO3(); name != "" && name != "und" {
		return name
	}
	return ""
}
