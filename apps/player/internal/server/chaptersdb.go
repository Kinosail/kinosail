package server

import "github.com/MikeO7/kinosail/packages/metadata"

type chapterProvider = metadata.ChapterProvider

func newChapterProvider(rawURL string) *chapterProvider {
	return metadata.NewChapterProvider(rawURL)
}
