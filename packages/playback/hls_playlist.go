package playback

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/MikeO7/kinosail/packages/isobmff"
)

const maximumHLSPlaylistBytes = 1 << 20

type AtomicWriter func(string, []byte) error

func PublishVariants(ctx context.Context, source, directory, transcoder, codecs string, qualities []PlaybackQuality, results <-chan error, expected int, independent bool, write AtomicWriter) error { //nolint:cyclop,gocognit // Readiness and worker completion are one bounded coordination loop.
	if expected <= 0 || expected > 64 || write == nil {
		return errors.New("HLS publication configuration is invalid")
	}
	ticker := time.NewTicker(25 * time.Millisecond)
	defer ticker.Stop()
	completed, published := 0, false
	for completed < expected {
		if !published && VariantsReady(source, directory, qualities) {
			if err := WriteMaster(filepath.Join(directory, "index.m3u8"), transcoder, codecs, qualities, independent, write); err != nil {
				return err
			}
			published = true
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case err, open := <-results:
			if !open {
				return errors.New("HLS publication ended early")
			}
			completed++
			if err != nil {
				return err
			}
		case <-ticker.C:
		}
	}
	if !VariantsReady(source, directory, qualities) {
		return errors.New("transcoder produced no playable variants")
	}
	return WriteMaster(filepath.Join(directory, "index.m3u8"), transcoder, codecs, qualities, independent, write)
}

func VariantsReady(source, directory string, qualities []PlaybackQuality) bool {
	if len(qualities) == 0 {
		return false
	}
	for _, quality := range qualities {
		if !VariantReady(source, filepath.Join(directory, quality.Label)) {
			return false
		}
	}
	return true
}

func VariantReady(source, directory string) bool { //nolint:cyclop // Manifest parsing and asset checks form one readiness decision.
	playlist := filepath.Join(directory, "index.m3u8")
	if !Fresh(playlist, source) {
		return false
	}
	manifest, err := ReadHLSPlaylist(playlist)
	if err != nil {
		return false
	}
	initReady, segments := false, map[string]struct{}{}
	for _, line := range strings.Split(string(manifest), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, `#EXT-X-MAP:URI="`) {
			name := strings.TrimSuffix(strings.TrimPrefix(line, `#EXT-X-MAP:URI="`), `"`)
			if VariantFile(name) {
				_, err = os.Stat(filepath.Join(directory, name)) //nolint:gosec // Name passed the rendition allowlist.
				initReady = err == nil
			}
		} else if line != "" && !strings.HasPrefix(line, "#") && VariantFile(line) {
			_, err = os.Stat(filepath.Join(directory, line)) //nolint:gosec // Line passed the rendition allowlist.
			if err == nil {
				segments[line] = struct{}{}
			}
		}
	}
	return initReady && len(segments) > 0
}

func WriteMaster(path, transcoder, codecs string, qualities []PlaybackQuality, independent bool, write AtomicWriter) error { //nolint:cyclop // Validation and manifest construction form one atomic publication input.
	if write == nil || len(qualities) == 0 || len(transcoder) > 1024 || strings.ContainsAny(transcoder, "\r\n") || len(codecs) > 1024 || strings.ContainsAny(codecs, "\r\n\"") {
		return errors.New("HLS master playlist input is invalid")
	}
	manifest := []byte("#EXTM3U\n#KINOSAIL-TRANSCODER:" + transcoder + "\n#EXT-X-VERSION:7\n")
	if independent {
		manifest = append(manifest, "#EXT-X-INDEPENDENT-SEGMENTS\n"...)
	}
	for _, quality := range qualities {
		line, err := masterVariant(path, quality)
		if err != nil {
			return err
		}
		manifest = append(manifest, line...)
	}
	return write(path, manifest)
}

func VariantBandwidth(directory string, fallback int64) (int64, int64) {
	manifest, err := ReadHLSPlaylist(filepath.Join(directory, "index.m3u8"))
	if err != nil {
		return fallback, fallback * 11 / 10
	}
	var duration, total float64
	peak, segmentDuration := float64(0), float64(0)
	for _, line := range strings.Split(string(manifest), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "#EXTINF:") {
			segmentDuration, _ = strconv.ParseFloat(strings.TrimSuffix(strings.TrimPrefix(line, "#EXTINF:"), ","), 64)
		} else if segmentDuration > 0 && VariantFile(line) {
			info, statErr := os.Stat(filepath.Join(directory, line)) //nolint:gosec // Line passed the rendition allowlist.
			if statErr == nil {
				bits := float64(info.Size() * 8)
				total, duration = total+bits, duration+segmentDuration
				peak = max(peak, bits/segmentDuration)
			}
			segmentDuration = 0
		}
	}
	if duration == 0 {
		return fallback, fallback * 11 / 10
	}
	return max(1, int64(total/duration)), max(1, int64(peak))
}

func Fresh(playlist, source string) bool {
	manifest, manifestErr := os.Stat(playlist) //nolint:gosec // Callers use installation cache paths.
	media, mediaErr := os.Stat(source)         //nolint:gosec // Source comes from the scanned library.
	return manifestErr == nil && mediaErr == nil && manifest.Size() > 0 && !manifest.ModTime().Before(media.ModTime())
}

func MasterFresh(playlist, source, transcoder string) bool {
	_, fresh := freshMasterManifest(playlist, source, transcoder)
	return fresh
}

func freshMasterManifest(playlist, source, transcoder string) ([]byte, bool) {
	if !Fresh(playlist, source) {
		return nil, false
	}
	manifest, err := ReadHLSPlaylist(playlist)
	if err != nil {
		return nil, false
	}
	if !PlaylistHas(manifest, "#KINOSAIL-TRANSCODER:"+transcoder) {
		return nil, false
	}
	if !bytes.Contains(manifest, []byte("#EXT-X-STREAM-INF:")) {
		return nil, false
	}
	return manifest, true
}

func CacheFresh(playlist, source, transcoder string) bool {
	manifest, fresh := freshMasterManifest(playlist, source, transcoder)
	if !fresh {
		return false
	}
	variants := 0
	for _, line := range strings.Split(string(manifest), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.Split(line, "/")
		if len(parts) != 2 || !QualityDirectory(parts[0]) || parts[1] != "index.m3u8" || !FinalizedVariant(filepath.Join(filepath.Dir(playlist), parts[0])) {
			return false
		}
		variants++
	}
	return variants > 0
}

func SeekCacheFresh(directory, source, transcoder string) bool {
	if !MasterFresh(filepath.Join(directory, "index.m3u8"), source, transcoder) {
		return false
	}
	marker, err := os.ReadFile(filepath.Join(directory, ".seekable")) //nolint:gosec // Marker is below a validated cache directory.
	return err == nil && string(marker) == transcoder
}

func FinalizedVariant(directory string) bool { //nolint:cyclop,gocognit // Playlist and asset validation are one trust-boundary check.
	manifest, err := ReadHLSPlaylist(filepath.Join(directory, "index.m3u8"))
	if err != nil || !PlaylistHas(manifest, "#EXT-X-PLAYLIST-TYPE:EVENT") || !PlaylistHas(manifest, "#EXT-X-ENDLIST") {
		return false
	}
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
		info, statErr := os.Lstat(filepath.Join(directory, name)) //nolint:gosec // Name passed the rendition allowlist.
		if statErr != nil || !info.Mode().IsRegular() || info.Size() == 0 {
			return false
		}
	}
	return initReady && segments > 0
}

func PlaylistHas(manifest []byte, tag string) bool {
	for _, line := range strings.Split(string(manifest), "\n") {
		if strings.TrimSpace(line) == tag {
			return true
		}
	}
	return false
}

func SourceVersion(path string) string {
	info, err := os.Stat(path) //nolint:gosec // Path comes from the scanned library.
	if err != nil {
		return "missing"
	}
	return strconv.FormatInt(info.Size(), 10) + ":" + strconv.FormatInt(info.ModTime().UnixNano(), 10)
}

func FinalizePlaylist(playlist string) error {
	manifest, err := ReadHLSPlaylist(playlist)
	if err != nil {
		return err
	}
	if !PlaylistHas(manifest, "#EXT-X-PLAYLIST-TYPE:EVENT") || !PlaylistHas(manifest, "#EXT-X-ENDLIST") {
		return errors.New("transcoder produced an incomplete HLS playlist")
	}
	return nil
}

func masterVariant(path string, quality PlaybackQuality) (string, error) {
	if !validMasterQuality(quality) {
		return "", errors.New("HLS quality is invalid")
	}
	initialization, err := isobmff.Read(filepath.Join(filepath.Dir(path), quality.Label, "init.mp4"))
	if err != nil {
		return "", err
	}
	video, found := initialization.Video()
	if quality.Label == "audio" {
		if found || initialization.Codecs() != "mp4a.40.2" {
			return "", errors.New("HLS audio output is not AAC")
		}
		average, peak := VariantBandwidth(filepath.Join(filepath.Dir(path), quality.Label), quality.Bitrate)
		line := "#EXT-X-STREAM-INF:BANDWIDTH=" + strconv.FormatInt(peak, 10) + ",AVERAGE-BANDWIDTH=" + strconv.FormatInt(average, 10) + ",CODECS=\"mp4a.40.2\"\n" + quality.Label + "/index.m3u8\n"
		return line, nil
	}
	if !found {
		return "", errors.New("HLS output has no video configuration")
	}
	codecs := initialization.Codecs()
	quality.Width, quality.Height = video.Width, video.Height
	average, peak := VariantBandwidth(filepath.Join(filepath.Dir(path), quality.Label), quality.Bitrate)
	line := "#EXT-X-STREAM-INF:BANDWIDTH=" + strconv.FormatInt(peak, 10) + ",AVERAGE-BANDWIDTH=" + strconv.FormatInt(average, 10) + ",CODECS=\"" + codecs + "\",RESOLUTION=" + strconv.Itoa(quality.Width) + "x" + strconv.Itoa(quality.Height)
	if quality.FrameRate > 0 {
		line += ",FRAME-RATE=" + strings.TrimRight(strings.TrimRight(strconv.FormatFloat(quality.FrameRate, 'f', 3, 64), "0"), ".")
	}
	return line + ",VIDEO-RANGE=" + video.Range + ",CLOSED-CAPTIONS=NONE\n" + quality.Label + "/index.m3u8\n", nil
}

func validMasterQuality(quality PlaybackQuality) bool {
	if !QualityDirectory(quality.Label) || quality.Bitrate < 1 || quality.FrameRate < 0 {
		return false
	}
	if quality.Label == "audio" {
		return quality.Width == 0 && quality.Height == 0
	}
	return quality.Width >= 1 && quality.Height >= 1
}
