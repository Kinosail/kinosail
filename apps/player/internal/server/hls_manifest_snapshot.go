package server

import (
	"errors"
	"io"
	"os"
)

const maximumHLSManifestBytes = 1 << 20

// Only live encoder manifests admit complete atomic replacement. Copied
// metadata retains its strict pathname identity through copiedHLSCacheFile.
func readHLSManifest(root *os.Root) ([]byte, error) {
	inspected, err := root.Lstat("index.m3u8")
	if err != nil {
		return nil, hlsManifestError("path")
	}
	return readHLSManifestAfterInspection(root, inspected)
}

func readHLSManifestAfterInspection(root *os.Root, inspected os.FileInfo) ([]byte, error) {
	if !boundedHLSManifest(inspected) {
		return nil, hlsManifestError("path")
	}
	file, opened, err := openHLSManifestSnapshot(root)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return readHLSManifestFile(root, file, opened)
}

func readHLSManifestFile(root *os.Root, file *os.File, before os.FileInfo) ([]byte, error) {
	if !boundedHLSManifest(before) {
		return nil, hlsManifestError("descriptor")
	}
	data, err := io.ReadAll(io.LimitReader(file, maximumHLSManifestBytes+1))
	if err != nil {
		return nil, hlsManifestError("read")
	}
	after, statErr := file.Stat()
	if statErr != nil || int64(len(data)) != before.Size() || !sameCopiedHLSFile(before, after) {
		return nil, hlsManifestError("descriptor")
	}
	current, err := root.Lstat("index.m3u8")
	if err != nil || !boundedHLSManifest(current) {
		return nil, hlsManifestError("path")
	}
	return data, nil
}

func boundedHLSManifest(info os.FileInfo) bool {
	return info != nil && info.Mode().IsRegular() && info.Size() > 0 && info.Size() <= maximumHLSManifestBytes
}

type hlsManifestError string

func (failure hlsManifestError) Error() string { return errCopiedHLSIndex.Error() }
func (failure hlsManifestError) Unwrap() error { return errCopiedHLSIndex }

func hlsManifestFailureStage(err error) string {
	var failure hlsManifestError
	if errors.As(err, &failure) {
		switch failure {
		case "open", "descriptor", "read", "path":
			return string(failure)
		}
	}
	return "not_manifest"
}
