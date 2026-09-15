package server

import "github.com/MikeO7/kinosail/packages/mediaprobe"

func (probe *mediaProbe) enrichment() mediaprobe.Enrichment {
	result := mediaprobe.Enrichment{}
	if probe.chapters != nil {
		result.Chapters = probe.chapters.Chapters
	}
	if probe.markers != nil {
		result.Markers = probe.markers.Markers
	}
	return result
}
