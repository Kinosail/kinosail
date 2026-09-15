package server

import "github.com/MikeO7/kinosail/packages/transcodehardware"

func (store *settingsStore) setTranscoder(quality, codec, accelerator string, toneMap bool) error {
	return store.transcoderState().Set(store.hardware, transcodehardware.Selection{Transcoder: quality, Codec: codec, Accelerator: accelerator, ToneMap: toneMap}, store.editable)
}

func (store *settingsStore) reconcileHardwareSelection() error {
	return store.transcoderState().Reconcile(store.hardware, transcodehardware.ConfiguredSources(string(store.config.Source("transcoding.accelerator")), string(store.config.Source("transcoding.codec"))))
}
