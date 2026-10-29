package backup

import (
	"archive/tar"
	"io"
)

func writeArchive(writer io.Writer, version string, contents map[string][]byte, written []string) error {
	return writeArchiveLimit(writer, version, contents, written, maxArchiveSize, func(string) bool { return false })
}

func writeArchiveManifest(writer *tar.Writer, version string, written []string) error {
	return writeManifestData(writer, archiveManifest(version, written))
}
