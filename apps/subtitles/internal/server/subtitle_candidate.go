package server

import (
	"context"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/MikeO7/kinosail-subtitles/internal/subtitlelanguage"
	"github.com/MikeO7/kinosail/packages/library"
)

func (provider *subtitleProvider) subDLCandidateSearch(ctx context.Context, item library.Item, language string) subtitleCandidateSearch {
	if !provider.subDLConfigured() {
		return subtitleCandidateSearch{}
	}
	found, err := provider.searchSubDL(ctx, item, language)
	if err != nil {
		return subtitleCandidateSearch{Configured: true, Failed: true}
	}
	result := subtitleCandidateSearch{Candidates: make([]subtitleDownloadCandidate, 0, len(found)), Configured: true}
	for _, candidate := range found {
		link := candidate.URL
		result.Candidates = append(result.Candidates, subtitleDownloadCandidate{Language: language, Score: candidate.Score, ReleaseMatch: candidate.ReleaseMatch, Source: "subdl", HI: candidate.HI, Download: func(ctx context.Context) ([]byte, error) {
			return provider.download(ctx, link)
		}})
	}
	return result
}

const (
	minimumSubtitleScore = 60
	maximumDownloadTries = 3
)

type subDLResult struct {
	IMDBID string `json:"imdb_id"`
	TMDBID int64  `json:"tmdb_id"`
	Type   string `json:"type"`
	Name   string `json:"name"`
	Year   int    `json:"year"`
}

type subDLFile struct {
	Name        string `json:"name"`
	ReleaseName string `json:"release_name"`
	Language    string `json:"language"`
	URL         string `json:"url"`
	Season      int    `json:"season"`
	Episode     int    `json:"episode"`
	HI          bool   `json:"hi"`
}

type subDLSubtitle struct {
	Name           string      `json:"name"`
	ReleaseName    string      `json:"release_name"`
	Language       string      `json:"language"`
	URL            string      `json:"url"`
	Season         int         `json:"season"`
	Episode        int         `json:"episode"`
	EpisodeFrom    int         `json:"episode_from"`
	EpisodeEnd     int         `json:"episode_end"`
	FPS            string      `json:"fps"`
	HI             bool        `json:"hi"`
	FullSeason     bool        `json:"full_season"`
	ProductionType int         `json:"production_type"`
	UnpackFiles    []subDLFile `json:"unpack_files"`
}

type subDLResponse struct {
	Status    bool            `json:"status"`
	Results   []subDLResult   `json:"results"`
	Subtitles []subDLSubtitle `json:"subtitles"`
}

type subtitleCandidate struct {
	URL          string
	Release      string
	Language     string
	Season       int
	Episode      int
	HI           bool
	Machine      bool
	Score        int
	ReleaseMatch float64
}

func rankSubDLCandidates(item library.Item, language string, response subDLResponse) []subtitleCandidate {
	if !validSubDLResponse(response) {
		return nil
	}
	result := response.Results[0]
	if !subDLIdentityMatches(item, result) {
		return nil
	}
	candidates, valid := collectSubDLCandidates(item, language, result, response.Subtitles)
	if !valid {
		return nil
	}
	sortSubDLCandidates(candidates)
	return candidates[:min(len(candidates), maximumDownloadTries)]
}

func subDLCandidates(item library.Item, subtitle subDLSubtitle) ([]subtitleCandidate, bool) {
	if len(subtitle.UnpackFiles) > 100 {
		return nil, false
	}
	if item.Show != "" && len(subtitle.UnpackFiles) > 0 {
		candidates := make([]subtitleCandidate, 0, len(subtitle.UnpackFiles))
		for _, file := range subtitle.UnpackFiles {
			if file.Season == item.Season && file.Episode == item.Episode {
				candidates = append(candidates, subtitleCandidate{file.URL, firstNonempty(file.ReleaseName, file.Name), file.Language, file.Season, file.Episode, file.HI, subtitle.ProductionType == 3, 0, 0})
			}
		}
		return candidates, true
	}
	if subtitle.FullSeason {
		return nil, true
	}
	return []subtitleCandidate{{subtitle.URL, firstNonempty(subtitle.ReleaseName, subtitle.Name), subtitle.Language, subtitle.Season, subtitle.Episode, subtitle.HI, subtitle.ProductionType == 3, 0, 0}}, true
}

func appendCandidate(candidates []subtitleCandidate, item library.Item, language string, result subDLResult, candidate subtitleCandidate) []subtitleCandidate {
	score, releaseMatch, ok := scoreSubtitleCandidate(item, language, result, candidate)
	if !ok || score < minimumSubtitleScore {
		return candidates
	}
	candidate.Score, candidate.ReleaseMatch = score, releaseMatch
	return append(candidates, candidate)
}

func scoreSubtitleCandidate(item library.Item, language string, result subDLResult, candidate subtitleCandidate) (int, float64, bool) {
	if !validSubtitleCandidate(item, language, candidate) {
		return 0, 0, false
	}
	base := strings.TrimSuffix(filepath.Base(item.Path), filepath.Ext(item.Path))
	releaseMatch := subtitleReleaseSimilarity(base, candidate.Release)
	score := 20 + int(releaseMatch*20)
	ids, title, year, identityScore := subDLItemIdentity(item, candidate)
	score += identityScore
	score, anchored, valid := scoreSubDLIdentity(ids, result, score)
	if !valid {
		return 0, releaseMatch, false
	}
	titleMatch := subtitleTokenSimilarity(title, result.Name)
	if !anchored && titleMatch < 0.8 {
		return 0, releaseMatch, false
	}
	score += int(titleMatch * 15)
	yearScore, valid := subDLYearScore(year, result.Year)
	if !valid {
		return 0, releaseMatch, false
	}
	score += yearScore + subDLCandidateBonus(candidate)
	return min(score, 100), releaseMatch, true
}

func subDLItemIdentity(item library.Item, candidate subtitleCandidate) (map[string]string, string, string, int) {
	if item.Show == "" {
		title, year := subtitleSearchIdentity(item)
		return item.ProviderIDs, title, year, 0
	}
	score := 5
	if candidate.Season == item.Season && candidate.Episode == item.Episode {
		score += 20
	}
	return item.ShowProviderIDs, item.Show, item.ShowYear, score
}

func subDLYearScore(year string, resultYear int) (int, bool) {
	if year == "" || resultYear == 0 {
		return 0, true
	}
	if year != strconv.Itoa(resultYear) {
		return 0, false
	}
	return 5, true
}

func subDLCandidateBonus(candidate subtitleCandidate) int {
	score := 0
	if !candidate.HI {
		score += 5
	}
	if candidate.Machine {
		score -= 20
	}
	return score
}

func validSubtitleCandidate(item library.Item, language string, candidate subtitleCandidate) bool {
	if !validSubDLURL(candidate.URL) || !subtitleProviderLanguageMatches(language, candidate.Language, subtitlelanguage.SubDL) {
		return false
	}
	return item.Show == "" || (candidate.Season == 0 || candidate.Season == item.Season) && (candidate.Episode == 0 || candidate.Episode == item.Episode)
}

func validSubDLURL(value string) bool {
	if len(value) == 0 || len(value) > 2048 {
		return false
	}
	parsed, err := url.Parse(value)
	return err == nil && (!parsed.IsAbs() || parsed.Host != "")
}

func scoreSubDLIdentity(ids map[string]string, result subDLResult, score int) (int, bool, bool) {
	anchored := false
	if expected := ids["imdb"]; expected != "" {
		if !strings.EqualFold(expected, result.IMDBID) {
			return 0, false, false
		}
		score, anchored = score+30, true
	}
	if expected := ids["tmdb"]; expected != "" {
		if result.TMDBID == 0 || expected != strconv.FormatInt(result.TMDBID, 10) {
			return 0, false, false
		}
		if anchored {
			score += 5
		} else {
			score += 30
		}
		anchored = true
	}
	return score, anchored, true
}

func canonicalSubtitleLanguage(language string) string {
	if canonical, ok := subtitlelanguage.NormalizeLocal(language); ok {
		return canonical
	}
	return ""
}

func subtitleLanguageMatches(requested, reported string) bool {
	requested = canonicalSubtitleLanguage(requested)
	reported = canonicalSubtitleLanguage(reported)
	return subtitleLanguageIdentitiesMatch(requested, reported)
}

func subtitleProviderLanguageMatches(requested, reported string, provider subtitlelanguage.Provider) bool {
	requested = canonicalSubtitleLanguage(requested)
	if code, ok := subtitlelanguage.Code(requested, provider); ok && strings.EqualFold(strings.TrimSpace(reported), code) {
		return true
	}
	reported, ok := subtitlelanguage.NormalizeProvider(reported, provider)
	return ok && subtitleLanguageIdentitiesMatch(requested, reported)
}

func subtitleLanguageIdentitiesMatch(requested, reported string) bool {
	return requested != "" && reported != "" && (requested == reported || !strings.Contains(requested, "-") && strings.HasPrefix(reported, requested+"-"))
}

func subDLIdentityMatches(item library.Item, result subDLResult) bool {
	if item.Show != "" && result.Type != "" && result.Type != "tv" || item.Show == "" && result.Type != "" && result.Type != "movie" {
		return false
	}
	return true
}

func subtitleTokenSimilarity(left, right string) float64 {
	leftTokens, rightTokens := subtitleTokens(left), subtitleTokens(right)
	if len(leftTokens) == 0 || len(rightTokens) == 0 {
		return 0
	}
	intersection := 0
	for token := range leftTokens {
		if rightTokens[token] {
			intersection++
		}
	}
	union := len(leftTokens) + len(rightTokens) - intersection
	return float64(intersection) / float64(union)
}

func subtitleTokens(value string) map[string]bool {
	value = strings.ToLower(strings.NewReplacer(".", " ", "_", " ", "-", " ", "[", " ", "]", " ", "(", " ", ")", " ").Replace(value))
	ignored := map[string]bool{"srt": true, "vtt": true, "ass": true, "ssa": true, "ttml": true, "mkv": true, "mp4": true, "avi": true, "x264": true, "x265": true, "h264": true, "h265": true, "hevc": true, "aac": true, "dts": true, "bluray": true, "webrip": true, "web": true, "dl": true, "hdtv": true}
	tokens := make(map[string]bool)
	for _, token := range strings.Fields(value) {
		if !ignored[token] && len(token) > 1 {
			tokens[token] = true
		}
	}
	return tokens
}

func firstNonempty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
