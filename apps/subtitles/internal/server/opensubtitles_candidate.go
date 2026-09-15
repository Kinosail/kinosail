package server

import (
	"context"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/MikeO7/kinosail-subtitles/internal/subtitlelanguage"
	"github.com/MikeO7/kinosail/packages/library"
)

func (provider *subtitleProvider) openSubtitlesCandidateSearch(ctx context.Context, item library.Item, language string) subtitleCandidateSearch {
	if !provider.open.configured() {
		return subtitleCandidateSearch{}
	}
	found, err := provider.open.search(ctx, item, language)
	if err != nil {
		return subtitleCandidateSearch{Configured: true, Failed: true}
	}
	result := subtitleCandidateSearch{Candidates: make([]subtitleDownloadCandidate, 0, len(found)), Configured: true}
	for _, candidate := range found {
		fileID := candidate.FileID
		result.Candidates = append(result.Candidates, subtitleDownloadCandidate{Language: language, ExactHash: candidate.ExactHash, Score: candidate.Score, ReleaseMatch: candidate.ReleaseMatch, Source: "opensubtitles", HI: candidate.HI, Download: func(ctx context.Context) ([]byte, error) {
			return provider.open.download(ctx, fileID)
		}})
	}
	return result
}

type openSubtitlesResponse struct {
	TotalCount int                       `json:"total_count"`
	Data       []openSubtitlesSearchItem `json:"data"`
}

type openSubtitlesIdentity struct {
	IDs            map[string]string
	Title, Year    string
	IMDBID, TMDBID int64
	ProviderTitle  string
}

type openSubtitlesSearchItem struct {
	ID         string                  `json:"id"`
	Type       string                  `json:"type"`
	Attributes openSubtitlesAttributes `json:"attributes"`
}

type openSubtitlesAttributes struct {
	Language          string                      `json:"language"`
	HearingImpaired   bool                        `json:"hearing_impaired"`
	FromTrusted       bool                        `json:"from_trusted"`
	ForeignPartsOnly  bool                        `json:"foreign_parts_only"`
	AITranslated      bool                        `json:"ai_translated"`
	MachineTranslated bool                        `json:"machine_translated"`
	MoviehashMatch    bool                        `json:"moviehash_match"`
	Release           string                      `json:"release"`
	NbCD              int                         `json:"nb_cd"`
	Feature           openSubtitlesFeature        `json:"feature_details"`
	Files             []openSubtitlesSubtitleFile `json:"files"`
}

type openSubtitlesFeature struct {
	Type         string `json:"feature_type"`
	Year         int    `json:"year"`
	Title        string `json:"title"`
	MovieName    string `json:"movie_name"`
	IMDBID       int64  `json:"imdb_id"`
	TMDBID       int64  `json:"tmdb_id"`
	Season       int    `json:"season_number"`
	Episode      int    `json:"episode_number"`
	ParentIMDBID int64  `json:"parent_imdb_id"`
	ParentTitle  string `json:"parent_title"`
	ParentTMDBID int64  `json:"parent_tmdb_id"`
}

type openSubtitlesSubtitleFile struct {
	FileID   int64  `json:"file_id"`
	CDNumber int    `json:"cd_number"`
	Name     string `json:"file_name"`
}

type openSubtitlesCandidate struct {
	ExactHash    bool
	FileID       int64
	Score        int
	ReleaseMatch float64
	HI           bool
}

func rankOpenSubtitlesCandidates(item library.Item, language string, response openSubtitlesResponse) []openSubtitlesCandidate { //nolint:cyclop,gocognit // Provider response filtering keeps every trust and cardinality boundary together.
	if len(response.Data) == 0 || len(response.Data) > 50 || response.TotalCount < len(response.Data) {
		return nil
	}
	candidates := make([]openSubtitlesCandidate, 0, len(response.Data))
	for _, result := range response.Data {
		if result.Type != "subtitle" || len(result.Attributes.Files) != 1 || result.Attributes.NbCD > 1 || result.Attributes.ForeignPartsOnly {
			continue
		}
		file := result.Attributes.Files[0]
		if file.FileID <= 0 || file.CDNumber > 1 || len(file.Name) > 512 {
			continue
		}
		score, releaseMatch, ok := scoreOpenSubtitlesCandidate(item, language, result.Attributes, file.Name)
		if ok && score >= minimumSubtitleScore {
			candidates = append(candidates, openSubtitlesCandidate{result.Attributes.MoviehashMatch, file.FileID, score, releaseMatch, result.Attributes.HearingImpaired})
		}
	}
	sort.SliceStable(candidates, func(left, right int) bool {
		if candidates[left].Score == candidates[right].Score {
			return candidates[left].ReleaseMatch > candidates[right].ReleaseMatch
		}
		return candidates[left].Score > candidates[right].Score
	})
	if len(candidates) > maximumDownloadTries {
		candidates = candidates[:maximumDownloadTries]
	}
	return candidates
}

func scoreOpenSubtitlesCandidate(item library.Item, language string, attributes openSubtitlesAttributes, fileName string) (int, float64, bool) {
	if !subtitleProviderLanguageMatches(language, attributes.Language, subtitlelanguage.OpenSubtitles) || attributes.MachineTranslated {
		return 0, 0, false
	}
	identity, valid := openSubtitlesCandidateIdentity(item, attributes.Feature)
	if !valid {
		return 0, 0, false
	}
	score, anchored, valid := scoreOpenSubtitlesIdentity(identity)
	if !valid {
		return 0, 0, false
	}
	return completeOpenSubtitlesScore(item, identity, attributes, fileName, score, anchored)
}

func completeOpenSubtitlesScore(item library.Item, identity openSubtitlesIdentity, attributes openSubtitlesAttributes, fileName string, score int, anchored bool) (int, float64, bool) {
	titleMatch := subtitleTokenSimilarity(identity.Title, identity.ProviderTitle)
	if !anchored && titleMatch < 0.8 {
		return 0, 0, false
	}
	score += int(titleMatch * 15)
	yearScore, valid := openSubtitlesYearScore(identity.Year, attributes.Feature.Year)
	if !valid {
		return 0, 0, false
	}
	score += yearScore
	base := strings.TrimSuffix(filepath.Base(item.Path), filepath.Ext(item.Path))
	releaseScore, releaseMatch := openSubtitlesReleaseScore(base, attributes.Release, fileName, attributes.MoviehashMatch)
	score += releaseScore + openSubtitlesTrustScore(attributes)
	return min(score, 100), releaseMatch, true
}

func openSubtitlesYearScore(year string, providerYear int) (int, bool) {
	if year == "" || providerYear == 0 {
		return 0, true
	}
	if year != strconv.Itoa(providerYear) {
		return 0, false
	}
	return 5, true
}

func openSubtitlesReleaseScore(base, release, fileName string, moviehashMatch bool) (int, float64) {
	releaseMatch := subtitleReleaseSimilarity(base, firstNonempty(release, fileName))
	score := int(releaseMatch * 20)
	if moviehashMatch {
		return score + 45, 1
	}
	return score, releaseMatch
}

func openSubtitlesTrustScore(attributes openSubtitlesAttributes) int {
	score := 0
	if attributes.FromTrusted {
		score += 5
	}
	if !attributes.HearingImpaired {
		score += 5
	}
	if attributes.AITranslated {
		score -= 10
	}
	return score
}

func openSubtitlesCandidateIdentity(item library.Item, feature openSubtitlesFeature) (openSubtitlesIdentity, bool) {
	title, year := subtitleSearchIdentity(item)
	identity := openSubtitlesIdentity{IDs: item.ProviderIDs, Title: title, Year: year, IMDBID: feature.IMDBID, TMDBID: feature.TMDBID, ProviderTitle: firstNonempty(feature.Title, feature.MovieName)}
	if item.Show == "" {
		return identity, feature.Type == "" || strings.EqualFold(feature.Type, "movie")
	}
	if !strings.EqualFold(feature.Type, "episode") || feature.Season != item.Season || feature.Episode != item.Episode {
		return openSubtitlesIdentity{}, false
	}
	identity.IDs, identity.Title, identity.Year = item.ShowProviderIDs, item.Show, item.ShowYear
	identity.IMDBID, identity.TMDBID, identity.ProviderTitle = feature.ParentIMDBID, feature.ParentTMDBID, feature.ParentTitle
	return identity, true
}

func scoreOpenSubtitlesIdentity(identity openSubtitlesIdentity) (int, bool, bool) {
	anchored, score := false, 20
	if expected := numericProviderID(identity.IDs["imdb"]); expected != "" {
		if expected != strconv.FormatInt(identity.IMDBID, 10) {
			return 0, false, false
		}
		anchored, score = true, score+30
	}
	if expected := numericProviderID(identity.IDs["tmdb"]); expected != "" {
		if expected != strconv.FormatInt(identity.TMDBID, 10) {
			return 0, false, false
		}
		if anchored {
			score += 5
		} else {
			anchored, score = true, score+30
		}
	}
	return score, anchored, true
}

func numericProviderID(value string) string {
	value = strings.TrimPrefix(strings.ToLower(strings.TrimSpace(value)), "tt")
	value = strings.TrimLeft(value, "0")
	if value == "" {
		return ""
	}
	for _, character := range value {
		if character < '0' || character > '9' {
			return ""
		}
	}
	return value
}
