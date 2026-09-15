package server

import (
	"bytes"
	"encoding/json"
	"errors"

	"github.com/MikeO7/kinosail-subtitles/internal/settingsstate"
)

type installationSettingsAlias installationSettings

func (settings *installationSettings) UnmarshalJSON(data []byte) error {
	preferences, err := settingsstate.Parse(data)
	if err != nil {
		return err
	}
	decoded := installationSettingsAlias(*settings)
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err = decoder.Decode(&decoded); err != nil {
		return errors.New("invalid installation settings")
	}
	decoded.SubtitleLanguage, decoded.SubtitleLanguages = preferences.Primary, preferences.Languages
	*settings = installationSettings(decoded)
	return nil
}
