package server

import (
	"context"
	"net/url"
	"strings"

	"github.com/MikeO7/kinosail/packages/library"
)

type subtitleSource interface {
	name() string
	configured() bool
	checkCredentials(ctx context.Context) error
	candidates(ctx context.Context, item library.Item, language string) ([]subtitleDownloadCandidate, error)
}

func (provider *subtitleProvider) sources() []subtitleSource {
	return []subtitleSource{provider.subdl, provider.open, provider.subsource}
}

func searchSubtitleSource(ctx context.Context, source subtitleSource, item library.Item, language string) subtitleCandidateSearch {
	if !source.configured() {
		return subtitleCandidateSearch{}
	}
	found, err := source.candidates(ctx, item, language)
	if err != nil {
		return subtitleCandidateSearch{Configured: true, Failed: true}
	}
	return subtitleCandidateSearch{Candidates: found, Configured: true}
}

func (provider *openSubtitlesProvider) candidates(ctx context.Context, item library.Item, language string) ([]subtitleDownloadCandidate, error) {
	found, err := provider.search(ctx, item, language)
	if err != nil {
		return nil, err
	}
	candidates := make([]subtitleDownloadCandidate, 0, len(found))
	for _, candidate := range found {
		fileID := candidate.FileID
		candidates = append(candidates, subtitleDownloadCandidate{Language: language, ExactHash: candidate.ExactHash, Score: candidate.Score, ReleaseMatch: candidate.ReleaseMatch, Source: "opensubtitles", HI: candidate.HI, Download: func(ctx context.Context) ([]byte, error) {
			return provider.download(ctx, fileID)
		}})
	}
	return candidates, nil
}

func (provider *subSourceProvider) candidates(ctx context.Context, item library.Item, language string) ([]subtitleDownloadCandidate, error) {
	found, err := provider.search(ctx, item, language)
	if err != nil {
		return nil, err
	}
	candidates := make([]subtitleDownloadCandidate, 0, len(found))
	for _, candidate := range found {
		candidates = append(candidates, subtitleDownloadCandidate{Language: language, Score: candidate.Score, ReleaseMatch: candidate.ReleaseMatch, Source: "subsource", Preserve: true, HI: candidate.HI, Download: func(ctx context.Context) ([]byte, error) {
			return provider.download(ctx, candidate, item)
		}})
	}
	return candidates, nil
}

func trustedProviderLink(base, link, domain string) bool {
	origin, originErr := url.Parse(base)
	target, targetErr := url.Parse(link)
	if originErr != nil || targetErr != nil || target.Host == "" {
		return false
	}
	if target.Host == origin.Host {
		return target.Scheme == origin.Scheme
	}
	host := strings.ToLower(target.Hostname())
	return domain != "" && target.Scheme == "https" && (host == domain || strings.HasSuffix(host, "."+domain))
}
