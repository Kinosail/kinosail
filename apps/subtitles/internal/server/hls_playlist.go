package server

import (
	"context"

	"github.com/MikeO7/kinosail/packages/playback"
)

func publishVariants(ctx context.Context, source, directory, transcoder, codecs string, qualities []PlaybackQuality, results <-chan error, expected int, independent bool) error {
	return playback.PublishVariants(ctx, source, directory, transcoder, codecs, qualities, results, expected, independent, writeAtomicFile)
}

func masterFresh(playlist, source, transcoder string) bool {
	return playback.MasterFresh(playlist, source, transcoder)
}

func cacheFresh(playlist, source, transcoder string) bool {
	return playback.CacheFresh(playlist, source, transcoder)
}
func sourceVersion(path string) string          { return playback.SourceVersion(path) }
func finalizePlaylist(playlist string) error    { return playback.FinalizePlaylist(playlist) }
func hlsFile(name string) bool                  { return playback.HLSFile(name) }
func qualityDirectory(name string) bool         { return playback.QualityDirectory(name) }
func variantFile(name string) bool              { return playback.VariantFile(name) }
func audioTrackIndex(value string) (int, error) { return playback.AudioTrackIndex(value) }
