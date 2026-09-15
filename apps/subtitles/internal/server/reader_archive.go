package server

import (
	"context"

	"github.com/MikeO7/kinosail/packages/library"
)

func archiveImages(ctx context.Context, item library.Item) []readerPage {
	images := library.ArchiveImages(ctx, item)
	pages := make([]readerPage, len(images))
	for index, image := range images {
		pages[index] = readerPage{Title: image.Title, URL: archiveURL(item.ID, image.Name)}
	}
	return pages
}
