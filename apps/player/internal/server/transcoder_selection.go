package server

import "github.com/MikeO7/kinosail/packages/transcodehardware"

func (store *settingsStore) transcoding() transcodeSettings {
	settings, _ := store.transcodingFor("")
	return settings
}

func (store *settingsStore) transcodingFor(codec string) (transcodeSettings, error) {
	return store.transcoderState().Settings(store.hardware, codec)
}

func (store *settingsStore) preferredVideoCodec(client []string) string {
	return store.transcoderState().PreferredCodec(store.hardware, client)
}

var normalizeAccelerator, normalizeCodecSetting = transcodehardware.NormalizeAccelerator, transcodehardware.NormalizeCodec
