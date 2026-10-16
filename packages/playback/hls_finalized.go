package playback

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const finalizedVariantMarkerBytes = 256

func FinalizedVariant(directory string) bool {
	manifest, err := ReadHLSPlaylist(filepath.Join(directory, "index.m3u8"))
	if err != nil || !PlaylistHas(manifest, "#EXT-X-PLAYLIST-TYPE:EVENT") || !PlaylistHas(manifest, "#EXT-X-ENDLIST") {
		return false
	}
	marker, path := finalizedVariantMarker(directory, manifest)
	if marker != "" && readFinalizedVariantMarker(path) == marker {
		return true
	}
	if !finalizedVariantFiles(directory, manifest) {
		return false
	}
	if marker != "" {
		writeFinalizedVariantMarker(path, marker)
	}
	return true
}

func finalizedVariantMarker(directory string, manifest []byte) (string, string) {
	if !QualityDirectory(filepath.Base(directory)) {
		return "", ""
	}
	info, err := os.Lstat(directory)
	if err != nil || !info.IsDir() {
		return "", ""
	}
	digest := sha256.Sum256(manifest)
	marker := "1 " + hex.EncodeToString(digest[:]) + " " + strconv.FormatInt(info.ModTime().UnixNano(), 10) + " " + strconv.FormatInt(info.Size(), 10) + "\n"
	return marker, filepath.Join(filepath.Dir(directory), ".finalized-"+filepath.Base(directory))
}

func readFinalizedVariantMarker(path string) string {
	file, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, finalizedVariantMarkerBytes))
	if err != nil {
		return ""
	}
	return string(data)
}

func writeFinalizedVariantMarker(path, marker string) {
	file, err := os.CreateTemp(filepath.Dir(path), ".finalized-*.pending")
	if err != nil {
		return
	}
	_, err = file.WriteString(marker)
	if closeErr := file.Close(); err == nil {
		err = closeErr
	}
	if err == nil {
		err = os.Rename(file.Name(), path)
	}
	if err != nil {
		_ = os.Remove(file.Name())
	}
}

func finalizedVariantFiles(directory string, manifest []byte) bool {
	initReady, segments := false, 0
	for _, line := range strings.Split(string(manifest), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") && !strings.HasPrefix(line, `#EXT-X-MAP:URI="`) {
			continue
		}
		name := strings.TrimSuffix(strings.TrimPrefix(line, `#EXT-X-MAP:URI="`), `"`)
		if strings.HasPrefix(line, `#EXT-X-MAP:URI="`) {
			initReady = line == `#EXT-X-MAP:URI="init.mp4"`
		} else {
			name, segments = line, segments+1
		}
		if !VariantFile(name) || name != "init.mp4" && !strings.HasPrefix(name, "segment-") {
			return false
		}
		info, statErr := os.Lstat(filepath.Join(directory, name))
		if statErr != nil || !info.Mode().IsRegular() || info.Size() == 0 {
			return false
		}
	}
	return initReady && segments > 0
}
