package server

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/MikeO7/kinosail/packages/library"
)

func budgetCandidate(source string, score int, hearingImpaired bool, downloads *[]string, text string) subtitleDownloadCandidate {
	return subtitleDownloadCandidate{
		Language: "en", ExactHash: true, Score: score, ReleaseMatch: 1, Source: source, HI: hearingImpaired,
		Download: func(context.Context) ([]byte, error) {
			*downloads = append(*downloads, text)
			if text == "" {
				return nil, errors.New("unavailable")
			}
			return []byte("1\n00:00:01,000 --> 00:00:02,000\n" + text + "\n"), nil
		},
	}
}

func TestQuotaBlockedSubDLCandidatesDoNotUseTheDownloadBudget(t *testing.T) {
	t.Parallel()
	provider := &subtitleProvider{ledger: openTestSubtitleLedger(t, t.TempDir())}
	provider.ledger.state.SubDLDay, provider.ledger.state.SubDLDownloads = time.Now().UTC().Format("2006-01-02"), subDLAutomaticDownloadLimit
	var downloads []string
	candidates := []subtitleDownloadCandidate{
		budgetCandidate("subdl", 95, false, &downloads, "SubDL one"),
		budgetCandidate("subdl", 94, false, &downloads, "SubDL two"),
		budgetCandidate("subdl", 93, false, &downloads, "SubDL three"),
		budgetCandidate("opensubtitles", 80, false, &downloads, "OpenSubtitles"),
	}
	cleaned, _, err := provider.downloadSubtitleCandidates(t.Context(), library.Item{Kind: "video"}, provider.preferredSubtitleCandidates(candidates), nil)
	if err != nil || len(downloads) != 1 || downloads[0] != "OpenSubtitles" || len(cleaned.Data) == 0 {
		t.Fatalf("downloads=%v err=%v", downloads, err)
	}
}

func TestDownloadBudgetCountsOnlyRealAttemptsAndKeepsSDHCandidates(t *testing.T) {
	t.Parallel()
	settings := &settingsStore{file: "settings.json", value: installationSettings{SubtitlePreference: "sdh"}, persist: func(string, any) error { return nil }}
	provider := &subtitleProvider{settings: settings, ledger: openTestSubtitleLedger(t, t.TempDir())}
	var downloads []string
	var candidates []subtitleDownloadCandidate
	for _, score := range []int{99, 98, 97, 96} {
		candidates = append(candidates, budgetCandidate("opensubtitles", score, false, &downloads, ""))
	}
	candidates = append(candidates, budgetCandidate("opensubtitles", 90, true, &downloads, "Captions"))
	ranked := provider.preferredSubtitleCandidates(candidates)
	if len(ranked) != len(candidates) || !ranked[0].HI {
		t.Fatalf("ranked %d candidates, first HI=%t", len(ranked), len(ranked) > 0 && ranked[0].HI)
	}
	if _, _, err := provider.downloadSubtitleCandidates(t.Context(), library.Item{Kind: "video"}, ranked, nil); err != nil || downloads[0] != "Captions" {
		t.Fatalf("downloads=%v err=%v", downloads, err)
	}
	downloads = nil
	failing := make([]subtitleDownloadCandidate, 0, 5)
	for _, score := range []int{99, 98, 97, 96, 95} {
		failing = append(failing, budgetCandidate("opensubtitles", score, true, &downloads, ""))
	}
	if _, _, err := provider.downloadSubtitleCandidates(t.Context(), library.Item{Kind: "video"}, failing, nil); err == nil || len(downloads) != maximumDownloadTries {
		t.Fatalf("failing downloads attempted=%d err=%v", len(downloads), err)
	}
}
