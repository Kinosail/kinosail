package server

import (
	"bytes"
	"io"
	"os"
	"path/filepath"

	"github.com/MikeO7/kinosail/packages/playback"
)

type copiedHLSLegacyRead struct {
	source     *os.File
	sourceInfo os.FileInfo
	entries    map[string]os.FileInfo
	master     []byte
	manifest   []byte
}

func (legacy *copiedHLSLegacyRead) bind(held *copiedAACGeneration) error {
	for _, metadata := range []struct {
		name string
		data []byte
	}{{".source", []byte(held.policy)}, {".copy-timeline", held.timelineData}, {".copy-clock", held.certificateData}} {
		data, err := legacy.read(held.root, metadata.name, maximumCopiedHLSTimelineBytes)
		if err != nil || !bytes.Equal(data, metadata.data) {
			return errCopiedHLSIndex
		}
	}
	var err error
	legacy.master, err = legacy.read(held.root, "index.m3u8", maximumCopiedHLSTimelineBytes)
	if err != nil || !copiedAACMasterAllowed(legacy.master, held.policy, held.certificate.Rendition) {
		return errCopiedHLSIndex
	}
	name := filepath.Join(held.certificate.Rendition, "index.m3u8")
	legacy.manifest, err = legacy.read(held.root, name, maximumCopiedHLSTimelineBytes)
	if err != nil || !playback.PlaylistHas(legacy.manifest, "#EXT-X-ENDLIST") {
		return errCopiedHLSIndex
	}
	if _, valid := copiedHLSManifest(legacy.manifest, held.timeline); !valid {
		return errCopiedHLSIndex
	}
	return legacy.completeAssets(held)
}

func (legacy *copiedHLSLegacyRead) read(root *os.Root, name string, limit int64) ([]byte, error) {
	file, info, err := copiedHLSOpenFile(root, name, limit)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, limit+1))
	after, statErr := root.Lstat(name)
	if err != nil || statErr != nil || int64(len(data)) != info.Size() || !sameCopiedHLSFile(info, after) {
		return nil, errCopiedHLSIndex
	}
	legacy.entries[name] = info
	return data, nil
}

func (legacy *copiedHLSLegacyRead) completeAssets(held *copiedAACGeneration) error {
	names := make([]string, 0, len(held.timeline.Keys)+1)
	names = append(names, "init.mp4")
	for number := range held.timeline.Keys {
		names = append(names, "segment-"+copiedHLSSegmentDigits(number)+".m4s")
	}
	for _, name := range names {
		file, info, err := copiedHLSOpenFile(held.media, name, 64<<20)
		if err != nil {
			return errCopiedHLSIndex
		}
		_ = file.Close()
		legacy.entries[filepath.Join(held.certificate.Rendition, name)] = info
	}
	return nil
}

func (legacy *copiedHLSLegacyRead) current(held *copiedAACGeneration) bool {
	if legacy.source == nil || !held.manager.copiedAACQualificationComplete(held.ctx, held.item, held.recipe, held.policy, legacy.sourceInfo, legacy.source) {
		return false
	}
	base, err := held.manager.baseHLSSettings(held.item, held.recipe)
	if err != nil {
		return false
	}
	if _, err := held.manager.copiedAACSettings(held.item, held.recipe, base); err != nil {
		return false
	}
	if _, err := held.root.Lstat(".startup"); !os.IsNotExist(err) {
		return false // The immutable reader cannot adopt speculative startup state.
	}
	if !copiedHLSBoundMetadata(held.root, held.timelineData, held.certificateData, held.policy) ||
		!held.manager.copiedHLSCanonicalGeneration(held.directory, held.certificate.Rendition, held.root, held.media) {
		return false
	}
	for name, before := range legacy.entries {
		after, err := held.root.Lstat(name)
		if err != nil || !sameCopiedHLSFile(before, after) {
			return false
		}
	}
	master, masterErr := copiedHLSCacheFile(held.root, "index.m3u8", maximumCopiedHLSTimelineBytes)
	manifest, manifestErr := copiedHLSCacheFile(held.media, "index.m3u8", maximumCopiedHLSTimelineBytes)
	return held.ctx.Err() == nil && masterErr == nil && manifestErr == nil &&
		bytes.Equal(master, legacy.master) && bytes.Equal(manifest, legacy.manifest)
}
