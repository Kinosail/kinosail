package playback

import (
	"bytes"
	"encoding/json"
	"errors"
	"path/filepath"

	"github.com/MikeO7/kinosail/packages/privatefile"
	"github.com/MikeO7/kinosail/packages/transcodepolicy"
)

// BindHLSEncoder keeps seek segments on the encoder that produced the existing
// initialization data, while cache readers use the unchanged playback policy.
func BindHLSEncoder(directory string, options transcodepolicy.Settings, resume bool) error {
	identity := struct {
		Accelerator, Encoder, Device, Codec, HDR string
		ToneMap                                  string `json:",omitempty"`
	}{options.Accelerator, options.Encoder, options.Device, options.Codec, options.OutputHDR, options.HardwareToneMap}
	data, err := json.Marshal(identity)
	if err != nil || len(data) > 1024 || identity.Encoder == "" {
		return errors.New("HLS encoder identity is invalid")
	}
	path := filepath.Join(directory, ".encoder")
	if resume {
		previous, err := privatefile.Read(path, 1024)
		if err != nil || !bytes.Equal(data, previous) {
			return errors.New("playback encoder changed; start a new compatible stream")
		}
		return nil
	}
	return privatefile.WriteCache(path, data)
}
