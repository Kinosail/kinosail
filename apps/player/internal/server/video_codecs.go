package server

import "github.com/MikeO7/kinosail/packages/transcodehardware"

type videoCodecCapability = transcodehardware.Codec

func enhanceTranscoderPage(page string) string {
	return transcodehardware.EnhanceSettingsPage(page)
}
