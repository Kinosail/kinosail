package playback

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"golang.org/x/sync/singleflight"
)

const (
	IFrameInterval     = 10
	iframeWidth        = 320
	iframeMaximumTime  = 43200
	iframeStreamFormat = `#EXT-X-I-FRAME-STREAM-INF:BANDWIDTH=64000,CODECS="avc1.4d401e",RESOLUTION=%dx%d,URI="iframes/index.m3u8"`
)

var errIFrameRequest = errors.New("I-frame request is invalid")

type IFrames struct {
	cache, ffmpeg string
	acquire       func(context.Context) (func(), error)
	generating    singleflight.Group
}

type IFrameRequest struct {
	ID, Source, File string
	Duration         float64
	Recipe           HLSRecipe
	Height           int
}

func NewIFrames(cache, ffmpeg string, acquire func(context.Context) (func(), error)) *IFrames {
	return &IFrames{cache: cache, ffmpeg: ffmpeg, acquire: acquire}
}

func IFrameHeight(width, height, rotation int) int {
	if rotation%180 != 0 {
		width, height = height, width
	}
	if width <= 0 || height <= 0 {
		return 0
	}
	return min(1024, max(2, int(math.Round(float64(iframeWidth)*float64(height)/float64(width)/2))*2))
}

func WithIFrameStream(manifest []byte, height int) []byte {
	if height <= 0 || !bytes.Contains(manifest, []byte("#EXT-X-STREAM-INF:")) || bytes.Contains(manifest, []byte("#EXT-X-I-FRAME-STREAM-INF:")) {
		return manifest
	}
	result := bytes.TrimRight(append([]byte(nil), manifest...), "\n")
	return append(result, []byte("\n"+fmt.Sprintf(iframeStreamFormat, iframeWidth, height)+"\n")...)
}

func IFramePlaylist(duration float64) []byte {
	var playlist strings.Builder
	playlist.WriteString("#EXTM3U\n#EXT-X-VERSION:7\n#EXT-X-TARGETDURATION:" + strconv.Itoa(IFrameInterval) + "\n#EXT-X-MEDIA-SEQUENCE:0\n#EXT-X-PLAYLIST-TYPE:VOD\n#EXT-X-I-FRAMES-ONLY\n#EXT-X-INDEPENDENT-SEGMENTS\n#EXT-X-MAP:URI=\"init.mp4\"\n")
	count := iframeCount(duration)
	for index := range count {
		second, length := index*IFrameInterval, float64(IFrameInterval)
		if index == count-1 {
			length = duration - float64(second)
		}
		playlist.WriteString("#EXTINF:" + strconv.FormatFloat(length, 'f', 3, 64) + ",\n" + strconv.Itoa(second) + ".m4s\n")
	}
	playlist.WriteString("#EXT-X-ENDLIST\n")
	return []byte(playlist.String())
}

func iframeCount(duration float64) int {
	return max(1, int(math.Ceil((duration-1)/IFrameInterval)))
}

func (frames *IFrames) Serve(writer http.ResponseWriter, request *http.Request, target IFrameRequest) error {
	total := HLSPlaybackDuration(target.Recipe, target.Duration)
	if frames == nil || target.Recipe.Offset != 0 || target.Height <= 0 || target.Source == "" || total <= 0 || total > iframeMaximumTime {
		return errIFrameRequest
	}
	if target.File == "index.m3u8" {
		writer.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
		writer.Header().Set("Cache-Control", "no-store")
		_, err := writer.Write(IFramePlaylist(total))
		return err
	}
	second, sequence := 0, uint32(0)
	if target.File != "init.mp4" {
		value, found := strings.CutSuffix(target.File, ".m4s")
		parsed, err := strconv.Atoi(value)
		if !found || err != nil || len(value) > 5 || parsed < 0 || parsed%IFrameInterval != 0 || parsed >= iframeCount(total)*IFrameInterval || strconv.Itoa(parsed) != value {
			return errIFrameRequest
		}
		index := parsed / IFrameInterval
		if index < 0 || index > iframeMaximumTime/IFrameInterval {
			return errIFrameRequest
		}
		second, sequence = parsed, uint32(index)+1
	}
	timeline := Timeline{SourceDuration: target.Duration, Duration: total, Omitted: target.Recipe.Omitted}
	millis := int(math.Round(timeline.SourceTime(float64(second)) * 1000))
	initialization, fragment, timescale, err := frames.frame(request.Context(), target, millis)
	if err != nil {
		return err
	}
	body := initialization
	if target.File != "init.mp4" {
		body, err = retimeIFrameFragment(fragment, uint64(second)*uint64(timescale), sequence)
		if err != nil {
			return err
		}
	}
	writer.Header().Set("Content-Type", "video/mp4")
	writer.Header().Set("Cache-Control", "private, max-age=3600")
	_, err = writer.Write(body)
	return err
}

func (frames *IFrames) frame(ctx context.Context, target IFrameRequest, millis int) ([]byte, []byte, uint32, error) {
	path := filepath.Join(frames.cache, "trickplay", target.ID, "iframes", strconv.Itoa(millis)+"-"+strconv.Itoa(target.Height)+".mp4")
	if !Fresh(path, target.Source) {
		result := frames.generating.DoChan(path, func() (any, error) {
			release, err := frames.acquire(ctx)
			if err != nil {
				return nil, err
			}
			defer release()
			if Fresh(path, target.Source) {
				return nil, nil
			}
			err = frames.encode(ctx, target, path, millis)
			if err != nil && ctx.Err() == nil {
				slog.LogAttrs(ctx, slog.LevelWarn, "I-frame preview unavailable", slog.String("item_id", target.ID), slog.Int("source_millis", millis), slog.String("failure_class", iframeFailureClass(err)))
			}
			return nil, err
		})
		select {
		case <-ctx.Done():
			return nil, nil, 0, ctx.Err()
		case outcome := <-result:
			if outcome.Err != nil {
				return nil, nil, 0, outcome.Err
			}
		}
	}
	data, err := readIFrameFile(path)
	if err != nil {
		return nil, nil, 0, err
	}
	return splitIFrameFile(data)
}

func (frames *IFrames) encode(ctx context.Context, target IFrameRequest, path string, millis int) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	scratch, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".*.tmp.mp4")
	if err != nil {
		return err
	}
	temporary := scratch.Name()
	if err = scratch.Close(); err == nil {
		ctx, cancel := context.WithTimeout(ctx, trickplayTimeout)
		defer cancel()
		position := strconv.Itoa(millis/1000) + "." + fmt.Sprintf("%03d", millis%1000)
		//nolint:gosec // G204: the executable is installation config and the source is scanned library content.
		err = exec.CommandContext(ctx, frames.ffmpeg, "-hide_banner", "-loglevel", "error", "-nostdin", "-y", "-ss", position, "-i", target.Source,
			"-map", "0:v:0", "-frames:v", "1", "-an", "-sn", "-dn", "-vf", "scale="+strconv.Itoa(iframeWidth)+":"+strconv.Itoa(target.Height)+",setsar=1",
			"-r", "1", "-c:v", "libx264", "-preset", "veryfast", "-profile:v", "main", "-level", "3.0", "-pix_fmt", "yuv420p",
			"-x264-params", "keyint=1:scenecut=0", "-video_track_timescale", "90000",
			"-f", "mp4", "-movflags", "frag_keyframe+empty_moov+default_base_moof", temporary).Run()
	}
	if err == nil {
		var data []byte
		if data, err = readIFrameFile(temporary); err == nil {
			_, _, _, err = splitIFrameFile(data)
		}
	}
	if err == nil {
		err = os.Rename(temporary, path)
	}
	if err != nil {
		_ = os.Remove(temporary)
	}
	return err
}

func readIFrameFile(path string) ([]byte, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if info.Size() <= 0 || info.Size() > maximumIFrameFileBytes {
		return nil, errIFrameContainer
	}
	return os.ReadFile(path)
}

func iframeFailureClass(err error) string {
	var exit *exec.ExitError
	switch {
	case errors.As(err, &exit):
		return "encoder-exit"
	case errors.Is(err, errIFrameContainer):
		return "invalid-output"
	case errors.Is(err, context.DeadlineExceeded):
		return "timeout"
	default:
		return "io"
	}
}
