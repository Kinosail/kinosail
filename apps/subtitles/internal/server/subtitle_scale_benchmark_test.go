package server

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/MikeO7/kinosail/packages/library"
)

func BenchmarkSubtitleDashboardProjectionSDH(b *testing.B) {
	media, now := b.TempDir(), time.Now()
	items := make([]library.Item, 5000)
	settings := newSettingsStore("", "", nil)
	settings.value.SubtitleLanguages, settings.value.SubtitlePreference = []string{"en"}, "sdh"
	provider := newSubtitleProvider(SubtitleConfig{}, b.TempDir(), "", nil, settings, "")
	contents := []byte("1\n00:00:01,000 --> 00:00:02,000\n[DOOR CLOSES] Ready\n")
	for position := range items {
		id := fmt.Sprintf("%016x", position+1)
		video := filepath.Join(media, id+".mp4")
		sidecar := filepath.Join(media, id+".en.srt")
		if err := os.WriteFile(sidecar, contents, 0o600); err != nil {
			b.Fatal(err)
		}
		items[position] = library.Item{ID: id, Kind: "video", Title: id, Path: video, Subtitles: []string{sidecar}}
		record := completeSubtitleRecord(sidecar, contents, subtitleRecord{Source: "opensubtitles", Role: "captions", Score: 90, CheckedAt: now.Unix(), Managed: true})
		provider.ledger.state.Records[subtitleRecordKey(id, "en")] = record
	}
	index := memoryLibraryIndex(items, true)
	provider.index = index
	manager := newSubtitleManager(index, settings, provider, nil)
	request := ownerRequest("/?view=library")
	options, err := subtitleDashboardQuery(request)
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	for b.Loop() {
		data, err := manager.projection(request, options)
		if err != nil || data.Ready != len(items) {
			b.Fatalf("ready=%d error=%v", data.Ready, err)
		}
	}
}

func BenchmarkSubtitleLedgerStoreLargeLibrary(b *testing.B) {
	ledger, now := benchmarkSubtitleLedger(b, b.TempDir()), time.Now()
	const pairs = 6000
	record := func(position int) subtitleRecord {
		id := fmt.Sprintf("%016x", position+1)
		return subtitleRecord{Fingerprint: subtitleFingerprint([]byte(id)), OriginalFingerprint: subtitleFingerprint([]byte("original" + id)), Source: "opensubtitles", Score: 90, CheckedAt: now.Unix(), InstalledAt: now.Unix(), Managed: true, Size: 64000, Modified: now.UnixNano(), ReleaseMatch: 0.75, TimingEvidence: "audio", Synchronization: "linear"}
	}
	for position := range pairs {
		ledger.state.Records[subtitleRecordKey(fmt.Sprintf("%016x", position+1), "en")] = record(position)
		ledger.state.Searches[subtitleSearchKey(fmt.Sprintf("%016x", position+1), "en", "standard")] = subtitleSearchRecord{Outcome: "installed", CheckedAt: now.Unix(), NextAt: now.Add(time.Hour).Unix()}
	}
	if err := ledger.save(); err != nil {
		b.Fatal(err)
	}
	b.ReportMetric(pairs, "pairs")
	position := 0
	for b.Loop() {
		if err := ledger.store(subtitleRecordKey(fmt.Sprintf("%016x", position%pairs+1), "en"), record(position%pairs)); err != nil {
			b.Fatal(err)
		}
		position++
	}
}

func benchmarkSubtitleLedger(b *testing.B, directory string) *subtitleLedger {
	return openTestSubtitleLedger(b, directory)
}
