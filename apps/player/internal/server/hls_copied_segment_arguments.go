package server

// Only a structurally valid, unbound nonkey presentation may use this producer.
// These arguments never grant a generated-asset or public-cache certificate.
func validCopiedHLSPendingProducer(timeline *copiedHLSTimeline) bool {
	return validCopiedHLSTimeline(timeline) && timeline.Strategy == copiedHLSPrerollStrategy &&
		timeline.Presentation != nil && timeline.Presentation.Proof == nil && timeline.Clock == nil
}

func copiedHLSPendingProducerRecipe(timeline *copiedHLSTimeline, recipe hlsRecipe) bool {
	if timeline == nil || timeline.Presentation == nil && timeline.Strategy != copiedHLSPrerollStrategy {
		return true
	}
	return validCopiedHLSPendingProducer(timeline) && recipe.mode == "remux" &&
		!recipe.dialogueBoost && !recipe.normalizeLoudness && len(recipe.omitted) == 0
}

func indexedCopiedHLSSegmentArguments(arguments []string, timeline *copiedHLSTimeline) []string {
	if timeline == nil {
		return arguments
	}
	pending := validCopiedHLSPendingProducer(timeline)
	for number := range arguments {
		if number+1 >= len(arguments) {
			continue
		}
		if arguments[number] == "-hls_time" {
			arguments[number+1] = "0.1"
			if pending {
				arguments[number+1] = "2"
			}
		}
		if pending && arguments[number] == "-hls_segment_options" {
			arguments[number+1] = "movflags=+skip_sidx:avoid_negative_ts=disabled:use_editlist=1"
		}
	}
	return arguments
}
