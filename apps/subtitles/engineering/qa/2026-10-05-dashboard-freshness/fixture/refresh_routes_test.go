package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"regexp"
)

var r16ItemPattern = regexp.MustCompile("^[a-f0-9]{16}$")

func (target *r16Target) routeAllowed(method, route string) bool {
	switch method {
	case http.MethodGet:
		switch route {
		case "/account", "/?view=library", "/api/v1/events",
			"/api/v1/subtitle-library?view=summary",
			"/api/v1/subtitle-library?view=library",
			"/api/v1/subtitle-library?view=history":
			return true
		}
	case http.MethodPost:
		switch route {
		case "/setup", "/account/mfa/enable", "/api/v1/subtitle-providers/test":
			return true
		}
	case http.MethodDelete:
		return route == "/api/v1/session"
	default:
		return false
	}
	item := target.registeredItem()
	if item == "" {
		return false
	}
	base := "/api/v1/subtitle-library/" + url.PathEscape(item)
	if method == http.MethodGet {
		return route == base+"/inspect?language=en"
	}
	return method == http.MethodPost && (route == base+"/preview" || route == base+"/apply")
}

func (target *r16Target) registeredItem() string {
	target.mu.Lock()
	defer target.mu.Unlock()
	return target.item
}

func r16RegisterActualCatalogue(ctx context.Context, target *r16Target, empty bool) error {
	response := target.request(ctx, http.MethodGet, "/api/v1/subtitle-library?view=library", nil, r16Owner)
	if !response.Complete || response.Status != http.StatusOK {
		return errors.New("actual R16 catalogue unavailable")
	}
	var data struct {
		Total *int `json:"total"`
		Items []struct {
			ID    string `json:"id"`
			Ready bool   `json:"ready"`
		} `json:"items"`
	}
	if json.Unmarshal(response.Body, &data) != nil || data.Total == nil || data.Items == nil {
		return errors.New("actual R16 catalogue invalid")
	}
	if empty {
		if *data.Total != 0 || len(data.Items) != 0 {
			return errors.New("actual R16 provider catalogue was not empty")
		}
		return nil
	}
	if *data.Total != 1 || len(data.Items) != 1 {
		return errors.New("actual R16 catalogue was not singular")
	}
	item := data.Items[0]
	if !item.Ready || !r16ItemPattern.MatchString(item.ID) {
		return errors.New("actual R16 item was not eligible")
	}
	target.mu.Lock()
	defer target.mu.Unlock()
	if target.item != "" && target.item != item.ID {
		return errors.New("actual R16 item registration conflicted")
	}
	target.item = item.ID
	return nil
}
