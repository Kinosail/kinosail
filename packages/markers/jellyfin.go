package markers

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strconv"
	"strings"
)

type JellyfinSegment struct {
	ID, ItemID, Type     string
	StartTicks, EndTicks int64
}

func JellyfinHandler(resolve func(*http.Request) (string, []Marker, bool), write func(http.ResponseWriter, any)) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		if !validMarkerItemID(request.PathValue("id")) {
			http.NotFound(writer, request)
			return
		}
		itemID, markers, found := resolve(request)
		if !found {
			http.NotFound(writer, request)
			return
		}
		segments := JellyfinSegments(itemID, markers)
		write(writer, map[string]any{"Items": segments, "TotalRecordCount": len(segments), "StartIndex": 0})
	}
}

func JellyfinSegments(itemID string, markers []Marker) []JellyfinSegment {
	segments := make([]JellyfinSegment, 0, len(markers))
	for index, marker := range markers {
		if marker.Source != "manual" && marker.Source != "chapter" && marker.Source != "fingerprint" && marker.Source != "text" {
			continue
		}
		kind := map[string]string{"intro": "Intro", "recap": "Recap", "commercial": "Commercial", "outro": "Outro", "credits": "Outro"}[marker.Type]
		if kind == "" {
			continue
		}
		sum := sha256.Sum256([]byte(strings.Join([]string{itemID, marker.Type, marker.Source, strconv.Itoa(index)}, "\x00")))
		segments = append(segments, JellyfinSegment{hex.EncodeToString(sum[:16]), itemID, kind, int64(marker.Start * 1e7), int64(marker.End * 1e7)})
	}
	return segments
}
