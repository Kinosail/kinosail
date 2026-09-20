package server

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/MikeO7/kinosail/packages/hlsmanifest"
)

const (
	maxHLSPlaylistBytes       = 1 << 20
	maxHLSPlaylistLines       = 16 << 10
	maxHLSPlaylistURIs        = 4096
	maxHLSPlaylistOutputBytes = 2 << 20
)

func serveHLSPlaylistWithSession(writer http.ResponseWriter, request *http.Request, file *os.File, duration float64) {
	playID := jellyfinPlaySessionID(request)
	manifest, err := io.ReadAll(io.LimitReader(file, maxHLSPlaylistBytes+1))
	if err != nil {
		localizedNotFound(writer, request)
		return
	}
	if len(manifest) > maxHLSPlaylistBytes {
		localizedError(writer, request, "playlist exceeds the size limit", http.StatusServiceUnavailable)
		return
	}
	validated, err := hlsPlaylistWithSession(manifest, "")
	if err != nil {
		localizedError(writer, request, "playlist contains an invalid media reference", http.StatusServiceUnavailable)
		return
	}
	if playID == "" || validPlaybackSession(playID) {
		validated, _ = hlsmanifest.CompleteVOD(validated, duration, 4)
	}
	rewritten, err := hlsPlaylistWithSession(validated, playID)
	if err != nil {
		localizedError(writer, request, "playlist contains an invalid media reference", http.StatusServiceUnavailable)
		return
	}
	writer.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
	writer.Header().Set("X-Content-Type-Options", "nosniff")
	//nolint:gosec // G705: every URI is a fixed relative HLS filename and the optional session is URL-escaped.
	_, _ = writer.Write(rewritten)
}

func hlsPlaylistWithSession(manifest []byte, playID string) ([]byte, error) {
	if bytes.Count(manifest, []byte{'\n'}) >= maxHLSPlaylistLines {
		return nil, errors.New("too many HLS playlist lines")
	}
	rewriteSession := validPlaybackSession(playID)
	content := string(manifest)
	query := ""
	if rewriteSession {
		query = "playSessionId=" + url.QueryEscape(playID)
		content = strings.Replace(content, "#EXT-X-PLAYLIST-TYPE:EVENT\n", "#EXT-X-PLAYLIST-TYPE:EVENT\n#EXT-X-START:TIME-OFFSET=0,PRECISE=YES\n", 1)
	}
	lines := strings.Split(content, "\n")
	uriCount := 0
	for index, line := range lines {
		rewritten, reference, err := rewriteHLSPlaylistLine(line, query, rewriteSession)
		if err != nil {
			return nil, err
		}
		if reference {
			uriCount++
			if uriCount > maxHLSPlaylistURIs {
				return nil, errors.New("too many HLS media references")
			}
		}
		lines[index] = rewritten
	}
	result := []byte(strings.Join(lines, "\n"))
	if len(result) > maxHLSPlaylistOutputBytes {
		return nil, errors.New("rewritten HLS playlist is too large")
	}
	return result, nil
}

func rewriteHLSPlaylistLine(line, query string, rewrite bool) (string, bool, error) {
	if line != "" && !strings.HasPrefix(line, "#") {
		if !validHLSManifestURI(line) {
			return "", false, errors.New("invalid HLS media reference")
		}
		if rewrite {
			line = hlsURIWithQuery(line, query)
		}
		return line, true, nil
	}
	return rewriteHLSTagLine(line, query, rewrite)
}

func rewriteHLSTagLine(line, query string, rewrite bool) (string, bool, error) {
	compactLine := strings.NewReplacer(" ", "", "\t", "").Replace(strings.ToUpper(line))
	uriAttributes := strings.Count(compactLine, "URI=")
	if uriAttributes > 0 && (uriAttributes != 1 || !strings.HasPrefix(line, `#EXT-X-MAP:URI="`) || !strings.HasSuffix(line, `"`) || strings.Count(line, `"`) != 2) {
		return "", false, errors.New("malformed HLS tag reference")
	}
	start := strings.Index(line, `URI="`)
	if start < 0 {
		return line, false, nil
	}
	start += len(`URI="`)
	end := strings.IndexByte(line[start:], '"')
	if end < 0 {
		return "", false, errors.New("unterminated HLS tag reference")
	}
	uri := line[start : start+end]
	if !validHLSManifestURI(uri) {
		return "", false, errors.New("invalid HLS tag reference")
	}
	if rewrite {
		line = line[:start] + hlsURIWithQuery(uri, query) + line[start+end:]
	}
	return line, true, nil
}

func validHLSManifestURI(uri string) bool {
	if variantFile(uri) {
		return true
	}
	parts := strings.Split(uri, "/")
	return len(parts) == 2 && qualityDirectory(parts[0]) && parts[1] == "index.m3u8"
}

func hlsURIWithQuery(uri, query string) string {
	if strings.Contains(uri, "?") {
		return uri + "&" + query
	}
	return uri + "?" + query
}
