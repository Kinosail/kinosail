package server

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/MikeO7/kinosail/packages/library"
)

type copiedAACGeneration struct {
	manager                       *hlsManager
	ctx                           context.Context
	item                          library.Item
	recipe                        hlsRecipe
	directory, policy             string
	root, media                   *os.Root
	timeline                      *copiedHLSTimeline
	certificate                   copiedHLSClockCertificate
	timelineData, certificateData []byte
	release                       func()
}

func (value *copiedAACGeneration) close() {
	if value.media != nil {
		_ = value.media.Close()
		value.media = nil
	}
	if value.root != nil {
		_ = value.root.Close()
		value.root = nil
	}
	if value.release != nil {
		release := value.release
		value.release = nil
		release()
	}
}

func (manager *hlsManager) openCopiedAACGeneration(parent context.Context, item library.Item, recipe hlsRecipe, directory string) (*copiedAACGeneration, error) {
	options, err := manager.hlsSettings(item, recipe)
	if err != nil || !copiedAACPolicyRequired(options.Cache) {
		return nil, errCopiedHLSIndex
	}
	ctx, release, err := manager.copiedHLSClockAdmission(parent)
	if err != nil {
		return nil, err
	}
	value := &copiedAACGeneration{manager: manager, ctx: ctx, item: item, recipe: recipe, directory: directory, policy: options.Cache, release: release}
	if err := value.open(ctx); err != nil {
		value.close()
		return nil, errCopiedHLSIndex
	}
	return value, nil
}

func (value *copiedAACGeneration) open(ctx context.Context) error {
	if err := value.openTimeline(ctx); err != nil {
		return err
	}
	if err := value.openCertificate(); err != nil {
		return err
	}
	var err error
	value.media, err = value.root.OpenRoot(value.certificate.Rendition)
	if err != nil || verifyCopiedHLSRenditionAssets(ctx, value.media, value.certificate) != nil ||
		verifyCopiedAACAssets(ctx, value.media, value.timeline) != nil || !value.current() {
		return errCopiedHLSIndex
	}
	return nil
}

func (value *copiedAACGeneration) openTimeline(ctx context.Context) error {
	var err error
	value.root, err = value.manager.openCopiedHLSRoot(value.directory)
	if err == nil {
		value.timeline, err = value.manager.readCopiedHLSTimelineRoot(ctx, value.directory, value.policy, value.root)
	}
	if err != nil || value.timeline.Clock == nil || value.timeline.AudioOrigin == nil {
		return errCopiedHLSIndex
	}
	return nil
}

func (value *copiedAACGeneration) openCertificate() error {
	var err error
	value.timelineData, err = copiedHLSCacheFile(value.root, ".copy-timeline", maximumCopiedHLSTimelineBytes)
	if err == nil {
		value.certificateData, err = copiedHLSCacheFile(value.root, ".copy-clock", 4096)
	}
	if err != nil {
		return errCopiedHLSIndex
	}
	value.certificate, err = decodeCopiedHLSCertificate(value.certificateData, value.timelineData)
	if err != nil || value.certificate.Version != 2 {
		return errCopiedHLSIndex
	}
	return nil
}

func (value *copiedAACGeneration) current() bool {
	data, err := json.Marshal(value.timeline)
	return err == nil && bytes.Equal(data, value.timelineData) && value.certificate.Version == 2 && value.certificate.Timeline == sha256.Sum256(value.timelineData) && value.ctx.Err() == nil &&
		value.manager.validateHLSPolicy(value.ctx, value.item, value.recipe, value.policy) == nil &&
		copiedHLSBoundMetadata(value.root, value.timelineData, value.certificateData, value.policy) &&
		value.manager.copiedHLSCanonicalGeneration(value.directory, value.certificate.Rendition, value.root, value.media)
}

func (value *copiedAACGeneration) nameAllowed(name string) bool {
	if !hlsFile(name) || filepath.Dir(name) != value.certificate.Rendition {
		return false
	}
	if filepath.Base(name) == "init.mp4" {
		return true
	}
	number, valid := hlsSegmentNumber(filepath.Base(name))
	return valid && number < len(value.timeline.Keys)
}

func (manager *hlsManager) copiedAACCacheAsset(ctx context.Context, item library.Item, recipe hlsRecipe, directory, name string) error {
	options, err := manager.hlsSettings(item, recipe)
	if err != nil {
		return err
	}
	if !copiedAACPolicyRequired(options.Cache) {
		if manager.copiedAACMarkerPresent(directory) {
			return errCopiedHLSIndex
		}
		return nil
	}
	value, err := manager.openCopiedAACGeneration(ctx, item, recipe, directory)
	if err != nil {
		return err
	}
	defer value.close()
	if !value.nameAllowed(name) || !value.current() {
		return errCopiedHLSIndex
	}
	return nil
}

func (manager *hlsManager) serveCopiedAACFile(writer http.ResponseWriter, request *http.Request, item library.Item, recipe hlsRecipe, name, key string) bool {
	options, err := manager.hlsSettings(item, recipe)
	if err != nil {
		localizedNotFound(writer, request)
		return true
	}
	if !copiedAACPolicyRequired(options.Cache) {
		if manager.copiedAACMarkerPresent(filepath.Join(manager.cache, key)) {
			localizedNotFound(writer, request)
			return true
		}
		return false
	}
	value, err := manager.openCopiedAACGeneration(request.Context(), item, recipe, filepath.Join(manager.cache, key))
	if err != nil {
		localizedNotFound(writer, request)
		return true
	}
	defer value.close()
	file, info, content, err := value.openAsset(name)
	if err != nil {
		localizedNotFound(writer, request)
		return true
	}
	defer file.Close()
	writer.Header().Set("Content-Type", "video/mp4")
	manager.adoptRecipeFile(request, key, filepath.Join(manager.cache, key, name))
	value.release()
	value.release = nil // The metadata/probe lease ends before network transfer.
	http.ServeContent(writer, request, filepath.Base(name), info.ModTime(), content)
	return true
}

func (value *copiedAACGeneration) assetContent(file *os.File, info os.FileInfo, name string) (io.ReadSeeker, error) {
	if name != "init.mp4" && name != "segment-00000.m4s" {
		return io.NewSectionReader(file, 0, info.Size()), nil
	}
	limit, expected := int64(64<<20), value.certificate.First
	if name == "init.mp4" {
		limit, expected = 2<<20, value.certificate.Initialization
	}
	data, err := io.ReadAll(io.LimitReader(file, limit+1))
	if err != nil || int64(len(data)) != info.Size() || int64(len(data)) > limit || sha256.Sum256(data) != expected {
		return nil, errCopiedHLSIndex
	}
	return bytes.NewReader(data), nil
}

func (value *copiedAACGeneration) openAsset(name string) (*os.File, os.FileInfo, io.ReadSeeker, error) {
	if !value.nameAllowed(name) {
		return nil, nil, nil, errCopiedHLSIndex
	}
	file, info, err := copiedHLSOpenFile(value.media, filepath.Base(name), 64<<20)
	if err != nil {
		return nil, nil, nil, err
	}
	content, err := value.assetContent(file, info, filepath.Base(name))
	if err != nil {
		_ = file.Close()
		return nil, nil, nil, err
	}
	after, statErr := value.media.Lstat(filepath.Base(name))
	if statErr != nil || !sameCopiedHLSFile(info, after) || !value.current() {
		_ = file.Close()
		return nil, nil, nil, errCopiedHLSIndex
	}
	return file, info, content, nil
}
