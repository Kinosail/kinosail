package metadata

import (
	"bytes"
	"context"
	"image/jpeg"
	"io"
	"net/http"
	"os"
	"sync"

	"github.com/MikeO7/kinosail/packages/privatefile"
)

const defaultBackdropURL = "https://image.tmdb.org/t/p/original"

const legacyBackdropWidth = 780

var backdropDecodeSlot = make(chan struct{}, 1)

type BackdropValidationError struct{ class string }

func (*BackdropValidationError) Error() string { return "metadata backdrop is invalid" }

func (failure *BackdropValidationError) FailureClass() string {
	switch failure.class {
	case "jpeg-header", "dimensions", "jpeg-decode":
		return failure.class
	default:
		return "invalid-artwork"
	}
}

func DownloadProviderBackdrop(ctx context.Context, client *http.Client, baseURL, path, target string) error {
	if baseURL != defaultImageURL {
		return DownloadProviderImage(ctx, client, baseURL, path, target)
	}
	return downloadProviderImage(ctx, client, defaultBackdropURL, path, target, validateBackdropJPEG)
}

func RetainProviderBackdrop(ctx context.Context, baseURL, cache, target string) bool {
	if ctx.Err() != nil || baseURL != defaultImageURL || CurrentArtwork(cache, target) == "" {
		return false
	}
	data, err := privatefile.Read(target, 8<<20)
	return err == nil && validateBackdropJPEG(ctx, data) == nil
}

type backdropIdentity struct {
	path     string
	size     int64
	modified int64
}

var legacyBackdrops = struct {
	sync.Mutex
	entries map[backdropIdentity]bool
}{entries: make(map[backdropIdentity]bool)}

func LegacyProviderBackdrop(baseURL, cache, path string) bool {
	if baseURL != defaultImageURL {
		return false
	}
	info, current := currentArtworkInfo(cache, path)
	if !current {
		return false
	}
	identity := backdropIdentity{path: path, size: info.Size(), modified: info.ModTime().UnixNano()}
	legacyBackdrops.Lock()
	legacy, found := legacyBackdrops.entries[identity]
	legacyBackdrops.Unlock()
	if found {
		return legacy
	}
	file, err := os.Open(path) // #nosec G304 -- currentArtworkInfo confines the path to the metadata cache.
	if err != nil {
		return false
	}
	defer func() { _ = file.Close() }()
	config, err := jpeg.DecodeConfig(io.LimitReader(file, 1<<20))
	legacy = err == nil && config.Width <= legacyBackdropWidth
	legacyBackdrops.Lock()
	if len(legacyBackdrops.entries) >= 16_384 {
		clear(legacyBackdrops.entries)
	}
	legacyBackdrops.entries[identity] = legacy
	legacyBackdrops.Unlock()
	return legacy
}

func validateBackdropJPEG(ctx context.Context, data []byte) error {
	config, err := jpeg.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return &BackdropValidationError{class: "jpeg-header"}
	}
	if !validBackdropDimensions(config.Width, config.Height) {
		return &BackdropValidationError{class: "dimensions"}
	}
	select {
	case backdropDecodeSlot <- struct{}{}:
		defer func() { <-backdropDecodeSlot }()
	case <-ctx.Done():
		return ctx.Err()
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if _, err := jpeg.Decode(bytes.NewReader(data)); err != nil {
		return &BackdropValidationError{class: "jpeg-decode"}
	}
	return ctx.Err()
}

func validBackdropDimensions(width, height int) bool {
	return width > 0 && height > 0 && width <= 32_768 && height <= 32_768 &&
		int64(width)*int64(height) <= 10_000_000
}
