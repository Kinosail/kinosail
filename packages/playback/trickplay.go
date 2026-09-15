package playback

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"time"

	"github.com/MikeO7/kinosail/packages/library"
)

type TrickplayDependencies struct {
	Cache, FFmpeg string
	Lookup        func(*http.Request, string) (library.Item, bool)
	NotFound      func(http.ResponseWriter, *http.Request)
	Unavailable   func(http.ResponseWriter, *http.Request)
	RecipePolicy  HLSRecipePolicy
}

type Trickplay struct{ dependencies TrickplayDependencies }

func NewTrickplay(dependencies TrickplayDependencies) (*Trickplay, error) {
	if dependencies.Lookup == nil || dependencies.NotFound == nil || dependencies.Unavailable == nil || dependencies.RecipePolicy.MaxBitrate <= 0 || dependencies.RecipePolicy.OffsetStepMilliseconds <= 0 {
		return nil, errors.New("trickplay dependencies are invalid")
	}
	return &Trickplay{dependencies: dependencies}, nil
}

func (frames *Trickplay) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /trickplay/{id}/{second}", frames.Serve)
}

func (frames *Trickplay) Serve(writer http.ResponseWriter, request *http.Request) { //nolint:cyclop // Validation and presentation mapping precede the cache side effect.
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
		if err := GenerateTrickplay(request.Context(), frames.dependencies.Cache, frames.dependencies.FFmpeg, item.Path, target, second); err != nil {
			frames.dependencies.Unavailable(writer, request)
			return
		}
	}
	writer.Header().Set("Content-Type", "image/jpeg")
	http.ServeFile(writer, request, target) //nolint:gosec // Target contains a scanned ID and validated integer.
}

func GenerateTrickplay(ctx context.Context, cache, ffmpeg, source, target string, second int) error {
	if cache == "" || ffmpeg == "" || source == "" || target == "" || second < 0 || second > 43200 {
		return errors.New("trickplay is not configured")
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
		return err
	}
	temporary := target + ".tmp.jpg"
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	err := exec.CommandContext(ctx, ffmpeg, "-hide_banner", "-loglevel", "error", "-y", "-ss", strconv.Itoa(second), "-i", source, "-frames:v", "1", "-vf", "scale=320:-2", "-q:v", "4", temporary).Run() //nolint:gosec // Executable is installation config and source is scanned content.
	if err == nil {
		err = os.Rename(temporary, target)
	}
	if err != nil {
		_ = os.Remove(temporary)
	}
	return err
}
