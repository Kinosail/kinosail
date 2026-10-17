package playback

import (
	"context"
	"errors"
	"path/filepath"
)

type AtomicWriter func(string, []byte) error

func PublishVariants(ctx context.Context, source, directory, transcoder string, qualities []PlaybackQuality, results <-chan error, expected int, independent bool, write AtomicWriter) error {
	return PublishVariantsObserved(ctx, source, directory, transcoder, qualities, results, expected, independent, write, nil)
}

func PublishVariantsObserved(ctx context.Context, source, directory, transcoder string, qualities []PlaybackQuality, results <-chan error, expected int, independent bool, write AtomicWriter, ready func()) error { //nolint:gocognit // Readiness and worker completion are one bounded coordination loop.
	if expected <= 0 || expected > 64 || write == nil {
		return errors.New("HLS publication configuration is invalid")
	}
	changes := watchHLSChanges(ctx, publicationDirectories(directory, qualities))
	defer changes.Close()
	completed, published, wake := 0, false, changes.changed
	for completed < expected {
		if !published && VariantsReady(source, directory, qualities) {
			if err := WriteMaster(filepath.Join(directory, "index.m3u8"), transcoder, qualities, independent, write); err != nil {
				return err
			}
			published, wake = true, nil
			changes.Close()
			if ready != nil {
				ready()
			}
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
		case <-wake:
		}
	}
	if !VariantsReady(source, directory, qualities) {
		return errors.New("transcoder produced no playable variants")
	}
	if err := WriteMaster(filepath.Join(directory, "index.m3u8"), transcoder, qualities, independent, write); err != nil {
		return err
	}
	if !published && ready != nil {
		ready()
	}
	return nil
}

func publicationDirectories(directory string, qualities []PlaybackQuality) []string {
	directories := []string{directory}
	for _, quality := range qualities {
		directories = append(directories, filepath.Join(directory, quality.Label))
	}
	return directories
}
