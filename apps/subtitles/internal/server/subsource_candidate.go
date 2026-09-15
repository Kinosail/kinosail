package server

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/MikeO7/kinosail-subtitles/internal/subtitlelanguage"
	"github.com/MikeO7/kinosail/packages/library"
)

func (provider *subtitleProvider) subSourceCandidateSearch(ctx context.Context, item library.Item, language string) subtitleCandidateSearch {
	if !provider.subsource.configured() {
		return subtitleCandidateSearch{}
	}
	found, err := provider.subsource.search(ctx, item, language)
	if err != nil {
		return subtitleCandidateSearch{Configured: true, Failed: true}
	}
	result := subtitleCandidateSearch{Candidates: make([]subtitleDownloadCandidate, 0, len(found)), Configured: true}
	for _, candidate := range found {
		candidate := candidate
		result.Candidates = append(result.Candidates, subtitleDownloadCandidate{Language: language, Score: candidate.Score, ReleaseMatch: candidate.ReleaseMatch, Source: "subsource", Preserve: true, HI: candidate.HI, Download: func(ctx context.Context) ([]byte, error) {
			return provider.subsource.download(ctx, candidate, item)
		}})
	}
	return result
}

type subSourceMovieResponse struct {
	Success bool             `json:"success"`
	Data    []subSourceMovie `json:"data"`
}

type subSourceMovie struct {
	MovieID        int64  `json:"movieId"`
	Title          string `json:"title"`
	AlternateTitle string `json:"alternateTitle"`
	Type           string `json:"type"`
	ReleaseYear    int    `json:"releaseYear"`
	IMDBID         string `json:"imdbId"`
	TMDBID         string `json:"tmdbId"`
	Season         int    `json:"season"`
	SubtitleCount  int    `json:"subtitleCount"`
}

type subSourceSubtitleResponse struct {
	Success    bool                                    `json:"success"`
	Data       []subSourceSubtitle                     `json:"data"`
	Pagination struct{ Page, Limit, Total, Pages int } `json:"pagination"`
}

type subSourceSubtitle struct {
	SubtitleID      int64                          `json:"subtitleId"`
	MovieID         int64                          `json:"movieId"`
	Language        string                         `json:"language"`
	ReleaseInfo     []string                       `json:"releaseInfo"`
	Files           int                            `json:"files"`
	Size            int64                          `json:"size"`
	HearingImpaired bool                           `json:"hearingImpaired"`
	ForeignParts    bool                           `json:"foreignParts"`
	ProductionType  string                         `json:"productionType"`
	Downloads       int                            `json:"downloads"`
	Rating          struct{ Good, Bad, Total int } `json:"rating"`
}

type subSourceCandidate struct {
	ID           int64
	Score        int
	ReleaseMatch float64
	HI           bool
}

func selectSubSourceMovie(item library.Item, ids map[string]string, title, year string, response subSourceMovieResponse) (subSourceMovie, bool) {
	if !response.Success || len(response.Data) == 0 || len(response.Data) > 30 {
		return subSourceMovie{}, false
	}
	best, bestScore := subSourceMovie{}, -1
	for _, movie := range response.Data {
		if !validSubSourceMovie(item, movie) {
			continue
		}
		score, ok := scoreSubSourceMovie(ids, title, year, movie)
		if !ok {
			continue
		}
		if score > bestScore {
			best, bestScore = movie, score
		}
	}
	return best, bestScore >= 0
}

func validSubSourceMovie(item library.Item, movie subSourceMovie) bool {
	if !validSubSourceMovieText(movie) || !validSubSourceMovieNumbers(movie) {
		return false
	}
	series := oneOf(strings.ToLower(movie.Type), "tv", "series", "tvseries")
	return series == (item.Show != "") && (!series || movie.Season == 0 || movie.Season == item.Season)
}

func validSubSourceMovieText(movie subSourceMovie) bool {
	return movie.MovieID > 0 && len(movie.Title) > 0 && len(movie.Title) <= 512 && len(movie.AlternateTitle) <= 512 && len(movie.IMDBID) <= 32 && len(movie.TMDBID) <= 32
}

func validSubSourceMovieNumbers(movie subSourceMovie) bool {
	return movie.ReleaseYear >= 0 && movie.ReleaseYear <= 9999 && movie.Season >= 0 && movie.Season <= 10_000 && movie.SubtitleCount >= 0
}

func scoreSubSourceMovie(ids map[string]string, title, year string, movie subSourceMovie) (int, bool) {
	titleMatch := max(subtitleTokenSimilarity(title, movie.Title), subtitleTokenSimilarity(title, movie.AlternateTitle))
	score, anchored, valid := scoreSubSourceMovieIDs(ids, movie, int(titleMatch*20))
	if !valid || !validSubSourceMovieIdentity(anchored, titleMatch, year, movie.ReleaseYear) {
		return 0, false
	}
	return score, true
}

func scoreSubSourceMovieIDs(ids map[string]string, movie subSourceMovie, score int) (int, bool, bool) {
	anchored := false
	if expected := strings.ToLower(ids["imdb"]); expected != "" {
		if !strings.EqualFold(expected, movie.IMDBID) {
			return 0, false, false
		}
		score, anchored = score+40, true
	}
	if expected := numericProviderID(ids["tmdb"]); expected != "" && numericProviderID(movie.TMDBID) != "" {
		if expected != numericProviderID(movie.TMDBID) {
			return 0, false, false
		}
		score, anchored = score+20, true
	}
	return score, anchored, true
}

func validSubSourceMovieIdentity(anchored bool, titleMatch float64, year string, releaseYear int) bool {
	return (anchored || titleMatch >= 0.8) && (year == "" || releaseYear == 0 || year == strconv.Itoa(releaseYear))
}

func rankSubSourceCandidates(item library.Item, movie subSourceMovie, language string, response subSourceSubtitleResponse) []subSourceCandidate {
	if !validSubSourceResponse(response) {
		return nil
	}
	base := strings.TrimSuffix(filepath.Base(item.Path), filepath.Ext(item.Path))
	candidates := make([]subSourceCandidate, 0, len(response.Data))
	for _, subtitle := range response.Data {
		candidate, ok := scoreSubSourceCandidate(item, movie, language, subtitle, base)
		if !ok {
			continue
		}
		candidates = append(candidates, candidate)
	}
	sort.SliceStable(candidates, func(left, right int) bool {
		return candidates[left].Score > candidates[right].Score || candidates[left].Score == candidates[right].Score && candidates[left].ReleaseMatch > candidates[right].ReleaseMatch
	})
	return candidates[:min(len(candidates), maximumDownloadTries)]
}

func validSubSourceResponse(response subSourceSubtitleResponse) bool {
	count := len(response.Data)
	return response.Success && count > 0 && count <= 50 && validSubSourcePagination(response.Pagination.Page, response.Pagination.Limit, response.Pagination.Total, response.Pagination.Pages, count)
}

func validSubSourcePagination(page, limit, total, pages, count int) bool {
	return page >= 1 && limit >= count && limit <= 100 && total >= count && pages >= 1
}

func scoreSubSourceCandidate(item library.Item, movie subSourceMovie, language string, subtitle subSourceSubtitle, base string) (subSourceCandidate, bool) {
	if !validSubSourceCandidateInput(movie, subtitle, language) {
		return subSourceCandidate{}, false
	}
	release := strings.Join(subtitle.ReleaseInfo, " ")
	if !validSubSourceRelease(item, subtitle, release) {
		return subSourceCandidate{}, false
	}
	releaseMatch := subSourceReleaseMatch(item, subtitle, base, release)
	score := 60 + int(releaseMatch*20) + min(subtitle.Rating.Good, 10)
	if !subtitle.HearingImpaired {
		score += 5
	}
	return subSourceCandidate{subtitle.SubtitleID, min(score, 100), releaseMatch, subtitle.HearingImpaired}, true
}

func validSubSourceCandidateInput(movie subSourceMovie, subtitle subSourceSubtitle, language string) bool {
	count := len(subtitle.ReleaseInfo)
	return validSubSourceSubtitle(movie, subtitle) && subtitleProviderLanguageMatches(language, subtitle.Language, subtitlelanguage.SubSource) && count > 0 && count <= 20
}

func validSubSourceRelease(item library.Item, subtitle subSourceSubtitle, release string) bool {
	return len(release) <= 2048 && (item.Show == "" || subtitle.Files != 1 || subSourceEpisodeName(release, item.Season, item.Episode))
}

func subSourceReleaseMatch(item library.Item, subtitle subSourceSubtitle, base, release string) float64 {
	match := subtitleReleaseSimilarity(base, release)
	if subtitle.Files > 1 && item.Show != "" || subSourceEpisodeName(release, item.Season, item.Episode) {
		return max(match, 0.8)
	}
	return match
}

func validSubSourceSubtitle(movie subSourceMovie, subtitle subSourceSubtitle) bool {
	return validSubSourceSubtitleFile(movie, subtitle) && validSubSourceSubtitleRating(subtitle) && !subtitle.ForeignParts && !oneOf(strings.ToLower(subtitle.ProductionType), "machine", "forced")
}

func validSubSourceSubtitleFile(movie subSourceMovie, subtitle subSourceSubtitle) bool {
	return subtitle.SubtitleID > 0 && subtitle.MovieID == movie.MovieID && subtitle.Files >= 1 && subtitle.Files <= 100 && subtitle.Size >= 1 && subtitle.Size <= 4<<20 && subtitle.Downloads >= 0
}

func validSubSourceSubtitleRating(subtitle subSourceSubtitle) bool {
	return subtitle.Rating.Good >= 0 && subtitle.Rating.Bad >= 0 && subtitle.Rating.Total >= subtitle.Rating.Good+subtitle.Rating.Bad
}

func subSourceEpisodeName(name string, season, episode int) bool {
	name = strings.ToLower(strings.NewReplacer(".", " ", "_", " ", "-", " ").Replace(name))
	return strings.Contains(name, strings.ToLower(fmt.Sprintf("s%02de%02d", season, episode))) || strings.Contains(name, strings.ToLower(fmt.Sprintf("s%de%d", season, episode)))
}

func subSourceLanguage(language string) (string, bool) {
	if !validLanguage(language) {
		return "", false
	}
	canonical, _ := subtitlelanguage.NormalizeTag(language)
	value, ok := subtitlelanguage.Code(canonical, subtitlelanguage.SubSource)
	if canonical == "pt-BR" {
		return "brazilian_portuguese", ok
	}
	return strings.ToLower(strings.ReplaceAll(value, " ", "_")), ok
}
