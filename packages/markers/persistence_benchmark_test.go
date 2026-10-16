package markers

import (
	"context"
	"errors"
	"testing"

	"github.com/MikeO7/kinosail/packages/documentdb"
	"github.com/MikeO7/kinosail/packages/library"
)

func BenchmarkDatabaseMarkerBatchPersistence(b *testing.B) {
	items := libraryMovies(500)
	for b.Loop() {
		b.StopTimer()
		directory := b.TempDir()
		store, err := documentdb.Open(directory, false, documentdb.Config{Documents: []string{"playback_markers.json"}})
		if err != nil {
			b.Fatal(err)
		}
		analyzer := NewAnalyzer(Config{DataDir: directory, Database: store})
		analyzer.SetProbe(func(context.Context, library.Item) Media { return Media{Duration: 7200} })
		analyzer.extract = func(context.Context, library.Item, string, float64, float64) ([]uint32, error) {
			return nil, errors.New("audio unavailable")
		}
		analyzer.extractVisual = func(context.Context, library.Item, string, float64, float64) ([]uint64, error) {
			return nil, errors.New("visual unavailable")
		}
		analyzer.extractCredits = func(context.Context, library.Item, float64, float64) ([]byte, error) { return nil, nil }
		b.StartTimer()
		if err := analyzer.analyzeBatch(b.Context(), items, nil); err != nil {
			b.Fatal(err)
		}
		b.StopTimer()
		if err := store.Close(); err != nil {
			b.Fatal(err)
		}
		b.StartTimer()
	}
}
