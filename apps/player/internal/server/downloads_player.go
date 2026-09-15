package server

import "github.com/MikeO7/kinosail/packages/library"

func optimizedDownloadLabel(item library.Item) string {
	if item.Kind == "video" {
		return "720p"
	}
	return "audio"
}

func supportsOffline(item library.Item) bool {
	return item.Kind == "video" || item.Kind == "audio" || item.Kind == "audiobook"
}
