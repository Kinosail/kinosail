package viewing

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

type jellyfinViewingUserData struct {
	Played                bool
	IsFavorite            bool
	PlaybackPositionTicks int64
	LastPlayedDate        string
}

type jellyfinViewingItem struct {
	ID, Name, Type, SeriesName, Path string
	ProductionYear                   int
	IndexNumber, ParentIndexNumber   int
	RunTimeTicks                     int64
	ProviderIDs                      map[string]string `json:"ProviderIds"`
	UserData                         jellyfinViewingUserData
}

type jellyfinViewingPage struct {
	Items            []jellyfinViewingItem
	TotalRecordCount int
}

func jellyfinViewingUserID(ctx context.Context, client *http.Client, input Input) (string, error) {
	if input.SourceUser != "" {
		return input.SourceUser, nil
	}
	var me struct {
		ID string `json:"Id"`
	}
	if err := viewingSourceJSON(ctx, client, input, "/Users/Me", nil, &me); err != nil {
		return "", errors.New("Jellyfin source user ID is required for this token") //nolint:staticcheck // Jellyfin is a proper product name.
	}
	me.ID = strings.TrimSpace(me.ID)
	if me.ID == "" || len(me.ID) > 512 {
		return "", errors.New("Jellyfin source user ID is invalid") //nolint:staticcheck // Jellyfin is a proper product name.
	}
	return me.ID, nil
}

func fetchJellyfinViewingPage(ctx context.Context, client *http.Client, input Input, userID string, start int) (jellyfinViewingPage, error) {
	query := url.Values{"UserId": {userID}, "Recursive": {"true"}, "IncludeItemTypes": {"Movie,Episode"}, "Fields": {"Path,ProviderIds"}, "EnableUserData": {"true"}, "EnableImages": {"false"}, "StartIndex": {strconv.Itoa(start)}, "Limit": {"200"}}
	var page jellyfinViewingPage
	if err := viewingSourceJSON(ctx, client, input, "/Items", query, &page); err != nil {
		return jellyfinViewingPage{}, err
	}
	if err := validateViewingPage(len(page.Items), page.TotalRecordCount); err != nil {
		return jellyfinViewingPage{}, err
	}
	return page, nil
}

func appendJellyfinViewingActivities(result []Activity, items []jellyfinViewingItem) ([]Activity, error) {
	for _, item := range items {
		kind := strings.ToLower(item.Type)
		if kind != "episode" && kind != "movie" {
			continue
		}
		if len(item.ProviderIDs) > 16 {
			return nil, errors.New("Jellyfin source returned too many provider identifiers") //nolint:staticcheck // Jellyfin is a proper product name.
		}
		updated, err := parseViewingTime(item.UserData.LastPlayedDate)
		if err != nil {
			return nil, err
		}
		activity, err := normalizeViewingActivity(Activity{SourceID: item.ID, Kind: kind, Title: item.Name, Year: yearNumber(item.ProductionYear), Show: item.SeriesName, Season: item.ParentIndexNumber, Episode: item.IndexNumber, Path: item.Path, ProviderIDs: normalizeProviderIDs(item.ProviderIDs), Seconds: float64(item.UserData.PlaybackPositionTicks) / 10000000, Duration: float64(item.RunTimeTicks) / 10000000, Watched: item.UserData.Played, Favorite: item.UserData.IsFavorite, Updated: updated})
		if err != nil {
			return nil, err
		}
		result = append(result, activity)
	}
	return result, nil
}
