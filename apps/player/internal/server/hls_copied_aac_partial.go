package server

import (
	"bytes"
	"os"
	"path/filepath"
)

// Partial reads retain certified bytes and missing-cut fences; they never adopt
// a startup marker, prepare missing media, or change producer/job ownership.
type copiedHLSLegacyPartial struct {
	startup bool
	missing []string
}

func (partial *copiedHLSLegacyPartial) bind(held *copiedAACGeneration) error {
	if err := partial.bindStartup(held); err != nil {
		return err
	}
	for number := -1; number < len(held.timeline.Keys); number++ {
		if held.ctx.Err() != nil {
			return errCopiedHLSIndex
		}
		name := "init.mp4"
		if number >= 0 {
			name = "segment-" + copiedHLSSegmentDigits(number) + ".m4s"
		}
		if err := partial.bindAsset(held, name, number); err != nil {
			return err
		}
	}
	return nil
}

func (partial *copiedHLSLegacyPartial) bindStartup(held *copiedAACGeneration) error {
	if _, err := held.root.Lstat(".startup"); os.IsNotExist(err) {
		return nil
	} else if err != nil {
		return errCopiedHLSIndex
	}
	data, err := held.legacy.read(held.ctx, held.root, ".startup", 1)
	if err != nil || !bytes.Equal(data, []byte("1")) {
		return errCopiedHLSIndex
	}
	partial.startup = true
	return nil
}

func (partial *copiedHLSLegacyPartial) bindAsset(held *copiedAACGeneration, name string, number int) error {
	// Only an explicitly missing later certified cut can be skipped. Once any
	// entry is present, disappearance, bad kind, or an open race rejects.
	if number > 0 {
		if _, err := held.media.Lstat(name); os.IsNotExist(err) {
			partial.missing = append(partial.missing, name)
			return nil
		} else if err != nil {
			return errCopiedHLSIndex
		}
	}
	file, info, err := copiedHLSOpenFile(held.media, name, 64<<20)
	if err != nil {
		return errCopiedHLSIndex
	}
	_ = file.Close()
	held.legacy.entries[filepath.Join(held.certificate.Rendition, name)] = info
	return nil
}

func (legacy *copiedHLSLegacyRead) startupCurrent(held *copiedAACGeneration) bool {
	if legacy.partial != nil {
		return legacy.partial.current(held)
	}
	_, err := held.root.Lstat(".startup")
	return os.IsNotExist(err)
}

func (partial *copiedHLSLegacyPartial) current(held *copiedAACGeneration) bool {
	if held.ctx.Err() != nil || !partial.startupCurrent(held) {
		return false
	}
	for _, name := range partial.missing {
		if held.ctx.Err() != nil {
			return false
		}
		if _, err := held.media.Lstat(name); !os.IsNotExist(err) {
			return false
		}
	}
	return held.ctx.Err() == nil
}

func (partial *copiedHLSLegacyPartial) startupCurrent(held *copiedAACGeneration) bool {
	if !partial.startup {
		_, err := held.root.Lstat(".startup")
		return os.IsNotExist(err)
	}
	data, err := copiedHLSCacheFile(held.root, ".startup", 1)
	return err == nil && bytes.Equal(data, []byte("1"))
}
