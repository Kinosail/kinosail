package server

import (
	"io"
	"os"
)

const maximumHLSManifestBytes = 1 << 20

// Encoder manifests are atomically replaced while cached fragments are served.
// Keep the opened snapshot stable; copied metadata retains its stricter current
// pathname binding through copiedHLSCacheFile.
func readHLSManifest(root *os.Root) ([]byte, error) {
	file, before, err := copiedHLSOpenFile(root, "index.m3u8", maximumHLSManifestBytes)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return readHLSManifestFile(root, file, before)
}

func readHLSManifestFile(root *os.Root, file *os.File, before os.FileInfo) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(file, maximumHLSManifestBytes+1))
	after, statErr := file.Stat()
	if err != nil || statErr != nil || int64(len(data)) != before.Size() || !sameCopiedHLSFile(before, after) {
		return nil, errCopiedHLSIndex
	}
	current, err := root.Lstat("index.m3u8")
	if err != nil || !boundedHLSManifest(current) {
		return nil, errCopiedHLSIndex
	}
	return data, nil
}

func boundedHLSManifest(info os.FileInfo) bool {
	return info != nil && info.Mode().IsRegular() && info.Size() > 0 && info.Size() <= maximumHLSManifestBytes
}
