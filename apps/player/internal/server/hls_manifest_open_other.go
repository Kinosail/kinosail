//go:build !linux && !darwin

package server

import "os"

// Preserve the existing supported-platform safety boundary. In particular,
// Windows still rejects atomic replacement between this helper's Lstat/open.
func openHLSManifestSnapshot(root *os.Root) (*os.File, os.FileInfo, error) {
	file, info, err := copiedHLSOpenFile(root, "index.m3u8", maximumHLSManifestBytes)
	if err != nil {
		return nil, nil, hlsManifestError("open")
	}
	return file, info, nil
}
