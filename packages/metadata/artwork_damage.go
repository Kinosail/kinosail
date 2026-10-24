package metadata

import (
	"bytes"
	"context"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"os"
	"sync"
)

var damagedArtwork = struct {
	sync.Mutex
	entries map[backdropIdentity]bool
}{entries: make(map[backdropIdentity]bool)}

func DamagedArtwork(cache, path string) bool {
	info, current := currentArtworkInfo(cache, path)
	if !current {
		return false
	}
	identity := backdropIdentity{path: path, size: info.Size(), modified: info.ModTime().UnixNano()}
	damagedArtwork.Lock()
	damaged, found := damagedArtwork.entries[identity]
	damagedArtwork.Unlock()
	if found {
		return damaged
	}
	damaged = !intactImage(path, info.Size())
	damagedArtwork.Lock()
	if len(damagedArtwork.entries) >= 65_536 {
		clear(damagedArtwork.entries)
	}
	damagedArtwork.entries[identity] = damaged
	damagedArtwork.Unlock()
	return damaged
}

func intactImage(path string, size int64) bool {
	if size < 64 || size > 32<<20 {
		return false
	}
	file, err := os.Open(path) // #nosec G304 -- currentArtworkInfo confines the path to the metadata cache.
	if err != nil {
		return false
	}
	defer func() { _ = file.Close() }()
	config, format, err := image.DecodeConfig(io.LimitReader(file, 1<<20))
	if err != nil || config.Width <= 0 || config.Height <= 0 {
		return false
	}
	if format != "jpeg" {
		return true
	}
	tail := make([]byte, 2)
	if _, err := file.ReadAt(tail, size-2); err != nil {
		return false
	}
	return bytes.Equal(tail, []byte{0xFF, 0xD9})
}

func validateProviderJPEG(ctx context.Context, data []byte) error {
	if len(data) < 64 || !bytes.HasSuffix(bytes.TrimRight(data, "\x00\r\n"), []byte{0xFF, 0xD9}) {
		return &BackdropValidationError{class: "jpeg-decode"}
	}
	config, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || !validBackdropDimensions(config.Width, config.Height) {
		return &BackdropValidationError{class: "dimensions"}
	}
	return ctx.Err()
}
