package server

import (
	"bytes"
	"context"
	"net/url"

	"github.com/MikeO7/kinosail/packages/playback"
)

func publishVariants(ctx context.Context, source, directory, transcoder string, qualities []PlaybackQuality, results <-chan error, expected int, independent bool) error {
	return playback.PublishVariantsObserved(ctx, source, directory, transcoder, qualities, results, expected, independent, writeAtomicFile, func() { hlsObservationFor(ctx).emit("media_ready", "") })
}

func masterFresh(playlist, source, transcoder string) bool {
	return playback.MasterFresh(playlist, source, transcoder)
}

func cacheFresh(playlist, source, transcoder string) bool {
	return playback.CacheFresh(playlist, source, transcoder)
}

func seekCacheFresh(directory, source, transcoder string) bool {
	return playback.SeekCacheFresh(directory, source, transcoder)
}

func sourceVersion(path string) string          { return playback.SourceVersion(path) }
func finalizePlaylist(playlist string) error    { return playback.FinalizePlaylist(playlist) }
func hlsFile(name string) bool                  { return playback.HLSFile(name) }
func audioTrackIndex(value string) (int, error) { return playback.AudioTrackIndex(value) }

func hlsPlaylistWithQuery(manifest []byte, query url.Values) []byte {
	encoded := query.Encode()
	var result bytes.Buffer
	result.Grow(len(manifest) + bytes.Count(manifest, []byte("\n"))*(len(encoded)+1))
	for index, line := range bytes.Split(manifest, []byte("\n")) {
		if index > 0 {
			result.WriteByte('\n')
		}
		if len(line) > 0 && line[0] != '#' {
			writeHLSURIWithQuery(&result, line, encoded)
			continue
		}
		start := bytes.Index(line, []byte(`URI="`))
		if start < 0 {
			result.Write(line)
			continue
		}
		start += len(`URI="`)
		end := bytes.IndexByte(line[start:], '"')
		if end < 0 {
			result.Write(line)
			continue
		}
		result.Write(line[:start])
		writeHLSURIWithQuery(&result, line[start:start+end], encoded)
		result.Write(line[start+end:])
	}
	return result.Bytes()
}

func writeHLSURIWithQuery(result *bytes.Buffer, uri []byte, query string) {
	result.Write(uri)
	if bytes.IndexByte(uri, '?') >= 0 {
		result.WriteByte('&')
	} else {
		result.WriteByte('?')
	}
	result.WriteString(query)
}
