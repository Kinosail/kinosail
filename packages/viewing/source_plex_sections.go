package viewing

import (
	"context"
	"errors"
	"net/http"
	"sync"
	"sync/atomic"
)

func fetchPlexSections(ctx context.Context, client *http.Client, input Input, sections []plexViewingSection) ([]Activity, error) { //nolint:cyclop,gocognit,funlen // Validation, bounded fetches, and ordered collection share one source boundary.
	for _, section := range sections {
		if _, err := plexSectionType(section.Key, section.Type); err != nil {
			return nil, err
		}
	}
	if len(sections) == 0 {
		return []Activity{}, nil
	}
	workCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	type sectionResult struct {
		items []Activity
		err   error
	}
	results, done := make([]sectionResult, len(sections)), make([]chan struct{}, len(sections))
	for index := range done {
		done[index] = make(chan struct{})
	}
	var count atomic.Int64
	var overBudget atomic.Bool
	reserve := func(items int) bool {
		if count.Add(int64(items)) > maximumViewingItems {
			overBudget.Store(true)
			cancel()
			return false
		}
		return true
	}
	jobs := make(chan int)
	var workers sync.WaitGroup
	for range min(3, len(sections)) {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for position := range jobs {
				section := sections[position]
				results[position].items, results[position].err = fetchPlexSectionReserved(workCtx, client, input, section.Key, section.Type, nil, reserve)
				close(done[position])
			}
		}()
	}
	go func() {
	dispatch:
		for position := range sections {
			select {
			case jobs <- position:
			case <-workCtx.Done():
				break dispatch
			}
		}
		close(jobs)
	}()
	activities := make([]Activity, 0)
	for position := range sections {
		select {
		case <-done[position]:
		case <-workCtx.Done():
			workers.Wait()
			if overBudget.Load() {
				return nil, errors.New("Plex source returned too many items") //nolint:staticcheck // Plex is a proper product name.
			}
			return nil, ctx.Err()
		}
		if results[position].err != nil {
			cancel()
			workers.Wait()
			if overBudget.Load() {
				return nil, errors.New("Plex source returned too many items") //nolint:staticcheck // Plex is a proper product name.
			}
			return nil, results[position].err
		}
		activities = append(activities, results[position].items...)
	}
	workers.Wait()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return activities, nil
}
