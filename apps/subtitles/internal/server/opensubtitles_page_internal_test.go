package server

import (
	"testing"

	"github.com/MikeO7/kinosail/packages/library"
)

func TestOpenSubtitlesFullResultPageStillProducesCandidates(t *testing.T) {
	t.Parallel()
	item := library.Item{Title: "Arrival", Year: "2016", Path: "Arrival.2016.mkv", ProviderIDs: map[string]string{"imdb": "tt2543164"}}
	response := openSubtitlesResponse{TotalCount: 240}
	for index := range 60 {
		attributes := openSubtitlesAttributes{
			Language: "en", Release: "Arrival.2016",
			Feature: openSubtitlesFeature{Type: "movie", Year: 2016, Title: "Arrival", IMDBID: 2543164},
			Files:   []openSubtitlesSubtitleFile{{FileID: int64(index + 1), CDNumber: 1, Name: "Arrival.en.srt"}},
		}
		response.Data = append(response.Data, openSubtitlesSearchItem{Type: "subtitle", Attributes: attributes})
	}
	candidates := rankOpenSubtitlesCandidates(item, "en", response)
	if len(candidates) == 0 || len(candidates) > maximumProviderCandidates {
		t.Fatalf("a 60-result page produced %d candidates", len(candidates))
	}
	for _, candidate := range candidates {
		if candidate.FileID > 50 {
			t.Fatalf("candidate %d came from beyond the first 50 results", candidate.FileID)
		}
	}
}
