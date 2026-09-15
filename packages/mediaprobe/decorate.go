package mediaprobe

import (
	"context"
	"runtime"
	"sync"

	"github.com/MikeO7/kinosail/packages/library"
)

// Decorate adds trusted embedded audio tags to library items.
func (probe *Probe) Decorate(ctx context.Context, items []library.Item, enrichment Enrichment) []library.Item {
	jobs := make(chan int)
	var wait sync.WaitGroup
	workers := min(max(1, runtime.GOMAXPROCS(0)-1), 4, len(items))
	wait.Add(workers)
	for range workers {
		go func() {
			defer wait.Done()
			for index := range jobs {
				probe.decorateItem(ctx, &items[index], enrichment)
			}
		}()
	}
	for index := range items {
		if items[index].Kind == "audio" || items[index].Kind == "audiobook" {
			if ctx.Err() != nil {
				close(jobs)
				wait.Wait()
				return items
			}
			jobs <- index
		}
	}
	close(jobs)
	wait.Wait()
	return items
}

func (probe *Probe) decorateItem(ctx context.Context, item *library.Item, enrichment Enrichment) {
	tags := probe.Inspect(ctx, *item, enrichment).Tags
	if !item.LocalTitle && tags.Title != "" {
		item.Title, item.LocalTitle = tags.Title, true
	}
	if item.Artist == "" {
		item.Artist = tags.Artist
	}
	if item.AlbumArtist == "" {
		item.AlbumArtist = tags.AlbumArtist
	}
	if item.Album == "" {
		item.Album = tags.Album
	}
	if item.Genres == "" {
		item.Genres = tags.Genres
	}
	if item.Disc == 0 {
		item.Disc = tags.Disc
	}
	if item.Track == 0 {
		item.Track = tags.Track
	}
}
