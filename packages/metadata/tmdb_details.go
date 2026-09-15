package metadata

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/MikeO7/kinosail/packages/library"
)

// TMDBDetails applies Player's optional collection and episode enrichment policy.
type TMDBDetails struct {
	BaseURL string
	Fetch   func(context.Context, string, any) error
}

func (provider TMDBDetails) TVMetadata(ctx context.Context, item library.Item, id int, name, date string, record *Record, poster *string) {
	record.Title, record.Year = name, Year(date)
	if item.Episode == 0 {
		return
	}
	var episode struct {
		ID        int    `json:"id"`
		Name      string `json:"name"`
		Overview  string `json:"overview"`
		AirDate   string `json:"air_date"`
		StillPath string `json:"still_path"`
	}
	path := fmt.Sprintf("%s/tv/%d/season/%d/episode/%d", strings.TrimRight(provider.BaseURL, "/"), id, item.Season, item.Episode)
	if provider.Fetch(ctx, path, &episode) == nil {
		record.Title, record.Plot, record.Year = fmt.Sprintf("S%02dE%02d · %s", item.Season, item.Episode, episode.Name), episode.Overview, Year(episode.AirDate)
		if episode.StillPath != "" {
			*poster = episode.StillPath
		}
		if record.ProviderIDs == nil {
			record.ProviderIDs = make(map[string]string)
		}
		record.ProviderIDs["tmdb"] = strconv.Itoa(episode.ID)
	}
}

func (provider TMDBDetails) MovieCollection(ctx context.Context, id int) string {
	var details struct {
		Collection *struct {
			Name string `json:"name"`
		} `json:"belongs_to_collection"`
	}
	_ = provider.Fetch(ctx, fmt.Sprintf("%s/movie/%d", strings.TrimRight(provider.BaseURL, "/"), id), &details)
	if details.Collection != nil {
		return details.Collection.Name
	}
	return ""
}
