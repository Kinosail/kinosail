package playback

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"

	"github.com/MikeO7/kinosail/packages/library"
)

type TrickplayDependencies struct {
	Cache, FFmpeg string
	Lookup        func(*http.Request, string) (library.Item, bool)
	Acquire       func(context.Context) (func(), error)
	NotFound      func(http.ResponseWriter, *http.Request)
	Unavailable   func(http.ResponseWriter, *http.Request)
	RecipePolicy  HLSRecipePolicy
}

const (
	trickplayTimeout  = 15 * time.Second
	trickplayMissTime = 30 * time.Second
)

var errEmptyTrickplayFrame = errors.New("encoder wrote an empty frame")

type Trickplay struct {
	dependencies TrickplayDependencies
	generating   singleflight.Group
	mu           sync.Mutex
	misses       map[string]time.Time
}

func NewTrickplay(dependencies TrickplayDependencies) (*Trickplay, error) {
	if dependencies.Lookup == nil || dependencies.Acquire == nil || dependencies.NotFound == nil || dependencies.Unavailable == nil || dependencies.RecipePolicy.MaxBitrate <= 0 || dependencies.RecipePolicy.OffsetStepMilliseconds <= 0 {
		return nil, errors.New("trickplay dependencies are invalid")
	}
	return &Trickplay{dependencies: dependencies}, nil
}

func (frames *Trickplay) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /trickplay/{id}/{second}", frames.Serve)
}

func (frames *Trickplay) Serve(writer http.ResponseWriter, request *http.Request) {
	if frames == nil || request == nil {
		http.Error(writer, "preview unavailable", http.StatusServiceUnavailable)
		return
	}
	item, found := frames.dependencies.Lookup(request, request.PathValue("id"))
	second, err := strconv.Atoi(request.PathValue("second"))
	if !found || item.Kind != "video" || err != nil || second < 0 || second > 43200 {
		frames.dependencies.NotFound(writer, request)
		return
	}
	if token := request.URL.Query().Get("playbackToken"); token != "" {
		recipe, tokenErr := ParseHLSRecipe(token, frames.dependencies.RecipePolicy)
		if tokenErr != nil || len(recipe.Omitted) == 0 {
			frames.dependencies.NotFound(writer, request)
			return
		}
		duration := 43200.0
		for _, omitted := range recipe.Omitted {
			duration -= omitted.End - omitted.Start
		}
		timeline := Timeline{SourceDuration: 43200, Duration: duration, Omitted: recipe.Omitted}
		second = int(timeline.SourceTime(float64(second)))
	}
	second -= second % 10
	target := filepath.Join(frames.dependencies.Cache, "trickplay", item.ID, strconv.Itoa(second)+".jpg")
	if !Fresh(target, item.Path) {
		err := errTrickplayMiss
		if !frames.missed(target) {
			err = frames.generate(request.Context(), item.ID, item.Path, target, second)
		}
		if errors.Is(err, errTrickplayMiss) {
			frames.dependencies.NotFound(writer, request)
			return
		}
		if err != nil {
			frames.dependencies.Unavailable(writer, request)
			return
		}
	}
	writer.Header().Set("Content-Type", "image/jpeg")
	http.ServeFile(writer, request, target)
}

func (frames *Trickplay) generate(ctx context.Context, id, source, target string, second int) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	result := frames.generating.DoChan(target, func() (any, error) {
		flight, cancel := context.WithTimeout(context.WithoutCancel(ctx), trickplayTimeout)
		defer cancel()
		release, acquireErr := frames.dependencies.Acquire(flight)
		if acquireErr != nil {
			return nil, acquireErr
		}
		defer release()
		if Fresh(target, source) {
			return nil, nil
		}
		err := GenerateTrickplay(flight, frames.dependencies.Cache, frames.dependencies.FFmpeg, source, target, second)
		if !encoderMiss(flight, err) {
			return nil, err
		}
		frames.rememberMiss(target)
		slog.LogAttrs(flight, slog.LevelWarn, "trickplay frame unavailable", slog.String("item_id", id), slog.Int("second", second), slog.String("failure_class", encoderMissClass(err)))
		return nil, errTrickplayMiss
	})
	select {
	case <-ctx.Done():
		return ctx.Err()
	case outcome := <-result:
		return outcome.Err
	}
}

func GenerateTrickplay(ctx context.Context, cache, ffmpeg, source, target string, second int) error {
	return generateFrame(ctx, cache, ffmpeg, source, target, second, 320)
}

func GenerateEpisodeStill(ctx context.Context, cache, ffmpeg, source, target string, second int) error {
	return generateFrame(ctx, cache, ffmpeg, source, target, second, 1920)
}

func generateFrame(ctx context.Context, cache, ffmpeg, source, target string, second, width int) error {
	if cache == "" || ffmpeg == "" || source == "" || target == "" || second < 0 || second > 43200 {
		return errors.New("trickplay is not configured")
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
		return err
	}
	scratch, err := os.CreateTemp(filepath.Dir(target), filepath.Base(target)+".*.tmp.jpg")
	if err != nil {
		return err
	}
	temporary := scratch.Name()
	if err := scratch.Close(); err != nil {
		_ = os.Remove(temporary)
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, trickplayTimeout)
	defer cancel()
	err = exec.CommandContext(ctx, ffmpeg, "-hide_banner", "-loglevel", "error", "-y", "-ss", strconv.Itoa(second), "-i", source, "-frames:v", "1", "-vf", "scale='min("+strconv.Itoa(width)+",iw)':-2", "-q:v", "4", temporary).Run() //nolint:gosec // Executable is installation config and source is scanned content.
	if err == nil {
		err = nonEmptyFrame(temporary)
	}
	if err == nil {
		err = os.Rename(temporary, target)
	}
	if err != nil {
		_ = os.Remove(temporary)
	}
	return err
}
