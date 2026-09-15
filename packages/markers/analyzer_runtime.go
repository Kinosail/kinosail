package markers

import (
	"context"

	"github.com/MikeO7/kinosail/packages/library"
)

func (analyzer *Analyzer) waiter() func(context.Context) error {
	analyzer.mu.RLock()
	defer analyzer.mu.RUnlock()
	return analyzer.wait
}

func (analyzer *Analyzer) inspect(ctx context.Context, item library.Item) Media {
	analyzer.mu.RLock()
	probe := analyzer.probe
	analyzer.mu.RUnlock()
	if probe == nil {
		return Media{}
	}
	return probe(ctx, item)
}
