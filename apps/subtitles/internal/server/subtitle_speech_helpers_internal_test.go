package server

import "io"

func processSubtitleAudioReference(reader io.Reader) (subtitleAudioReference, error) {
	return processSubtitleAudio(reader, subtitleWorkerCount(), nil)
}
