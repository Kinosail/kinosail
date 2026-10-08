package server

// This compact tuple records a measured clock, not generic source eligibility.
// The producer must bind its source rows, actual first packet and private assets.
type copiedHLSAudioProof struct {
	Codec, Profile                               string
	SampleRate, Channels                         int64
	Numerator, Denominator                       int64
	SourceClock, FirstPacket                     [32]byte
	FirstPTS, FirstNativeSample, RequestedSample int64
	TargetPTS, TargetNativeSample, TargetSamples int64
	LeadingSamples, SourcePhase                  int64
	MediaTime, OriginalMediaTime                 int64
}

func validCopiedHLSAudioProof(mapping *copiedHLSPresentation) bool {
	audio := mapping.Proof.Audio
	if audio == nil {
		return true // Structural video geometry still cannot grant cache admission.
	}
	return validCopiedHLSAudioFormat(audio) && validCopiedHLSAudioLimits(audio) &&
		validCopiedHLSAudioPhase(audio) && validCopiedHLSAudioRequested(mapping, audio) &&
		validCopiedHLSAudioEdit(audio)
}

func validCopiedHLSAudioFormat(audio *copiedHLSAudioProof) bool {
	return audio.Codec == "aac" && audio.Profile == "LC" && audio.SampleRate == 48000 &&
		audio.Channels == 2 && audio.Numerator == 1 &&
		(audio.Denominator == 1000 || audio.Denominator == 48000) &&
		audio.SourceClock != [32]byte{} && audio.FirstPacket != [32]byte{}
}

func validCopiedHLSAudioLimits(audio *copiedHLSAudioProof) bool {
	return audio.FirstPTS >= 0 && audio.FirstPTS <= 22*audio.Denominator &&
		audio.FirstNativeSample >= 0 && audio.FirstNativeSample <= 22*48000 &&
		audio.TargetPTS >= 0 && audio.TargetPTS <= 22*48000 &&
		audio.TargetNativeSample >= 0 && audio.TargetNativeSample <= 22*48000 &&
		audio.RequestedSample > 0 && audio.RequestedSample <= 20*48000
}

func validCopiedHLSAudioPhase(audio *copiedHLSAudioProof) bool {
	switch audio.LeadingSamples {
	case 1024:
		if audio.SourcePhase != 0 {
			return false
		}
	case 16:
		if audio.SourcePhase != -8 || audio.Denominator != 48000 {
			return false
		}
	default:
		return false
	}
	if !copiedHLSAudioOrdinal(audio.FirstNativeSample, audio.LeadingSamples) ||
		!copiedHLSAudioOrdinal(audio.TargetNativeSample, audio.LeadingSamples) {
		return false
	}
	clockError := audio.FirstPTS*48000 - (audio.FirstNativeSample+audio.SourcePhase)*audio.Denominator
	return clockError >= -24000 && clockError <= 24000 &&
		audio.TargetPTS == audio.TargetNativeSample+audio.SourcePhase
}

func copiedHLSAudioOrdinal(sample, leading int64) bool {
	if leading == 1024 {
		return sample%1024 == 0
	}
	return sample >= leading && (sample-leading)%1024 == 0
}

func validCopiedHLSAudioRequested(mapping *copiedHLSPresentation, audio *copiedHLSAudioProof) bool {
	return audio.TargetSamples == 1024 && audio.FirstNativeSample <= audio.TargetNativeSample &&
		mapping.RequestedMicros*48000 == audio.RequestedSample*1_000_000 &&
		audio.TargetPTS <= audio.RequestedSample && audio.RequestedSample < audio.TargetPTS+audio.TargetSamples
}

func validCopiedHLSAudioEdit(audio *copiedHLSAudioProof) bool {
	desired := audio.TargetNativeSample + audio.RequestedSample - audio.TargetPTS
	delta := audio.MediaTime - audio.OriginalMediaTime
	return audio.MediaTime >= 32 && audio.MediaTime <= 16*48000 &&
		audio.OriginalMediaTime >= 32 && audio.OriginalMediaTime <= 16*48000 &&
		audio.MediaTime == desired-audio.FirstNativeSample && delta >= -32 && delta <= 32
}
