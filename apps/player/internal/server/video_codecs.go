package server

import "github.com/MikeO7/kinosail/packages/transcodehardware"

type videoCodecCapability = transcodehardware.Codec

func enhanceTranscoderPage(page string) string {
	return transcodehardware.EnhanceSettingsPage(page)
}

func hlsCodecs(facts MediaFacts, recipe hlsRecipe, outputCodec string) string {
	audioCodec := ""
	if len(facts.Audio) > 0 {
		audioCodec = facts.Audio[0].Codec
	}
	return transcodehardware.HLSCodecs(facts.Video.Codec, audioCodec, recipe.mode, outputCodec, len(facts.Audio) > 0)
}
