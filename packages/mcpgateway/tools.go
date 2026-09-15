package mcpgateway

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type mcpMediaInput struct {
	Query string `json:"query,omitempty" jsonschema:"Words, title, year, genre, credited person, director, studio, artist, album, or show to match"`
	View  string `json:"view,omitempty" jsonschema:"all, list, unwatched, history, movies, shows, music, audiobooks, books, or photos"`
	Sort  string `json:"sort,omitempty" jsonschema:"title, added, or year"`
	Limit int    `json:"limit,omitempty" jsonschema:"Maximum results from 1 to 200; defaults to 50"`
}

type mcpPlaylistInput struct {
	Name string   `json:"name" jsonschema:"Playlist name containing 1 to 64 characters"`
	IDs  []string `json:"ids" jsonschema:"Ordered media IDs selected from search_media or recommendation_context"`
}

type mcpRecommendationOutput struct {
	History    mcpAPIOutput `json:"history"`
	Candidates mcpAPIOutput `json:"candidates"`
}

func (adapter *Gateway) manageAPI(ctx context.Context, _ *mcp.CallToolRequest, input mcpManageAPIInput) (*mcp.CallToolResult, mcpAPIOutput, error) {
	method, err := mcpAPIMethod(input.Method, true)
	if err != nil {
		return nil, mcpAPIOutput{}, err
	}
	if method == http.MethodGet && input.Body != nil {
		return nil, mcpAPIOutput{}, errors.New("GET management requests must not include a body")
	}
	output, err := adapter.callAPI(ctx, method, input.Path, input.Body, ManageAccess)
	return nil, output, err
}

func (adapter *Gateway) searchMedia(ctx context.Context, _ *mcp.CallToolRequest, input mcpMediaInput) (*mcp.CallToolResult, mcpAPIOutput, error) {
	path, limit, err := mcpMediaPath(input)
	if err != nil {
		return nil, mcpAPIOutput{}, err
	}
	output, err := adapter.callAPI(ctx, http.MethodGet, path, nil, ReadAccess)
	return nil, limitMCPItems(output, limit), err
}

func (adapter *Gateway) recommendationContext(ctx context.Context, _ *mcp.CallToolRequest, input mcpMediaInput) (*mcp.CallToolResult, mcpRecommendationOutput, error) {
	if input.View == "" {
		input.View = "unwatched"
	}
	path, limit, err := mcpMediaPath(input)
	if err != nil {
		return nil, mcpRecommendationOutput{}, err
	}
	history, err := adapter.callAPI(ctx, http.MethodGet, "/api/v1/history", nil, ReadAccess)
	if err != nil {
		return nil, mcpRecommendationOutput{}, err
	}
	candidates, err := adapter.callAPI(ctx, http.MethodGet, path, nil, ReadAccess)
	return nil, mcpRecommendationOutput{limitMCPItems(history, limit), limitMCPItems(candidates, limit)}, err
}

func (adapter *Gateway) createPlaylist(ctx context.Context, _ *mcp.CallToolRequest, input mcpPlaylistInput) (*mcp.CallToolResult, mcpAPIOutput, error) {
	output, err := adapter.callAPI(ctx, http.MethodPost, "/api/v1/playlists", input, WriteAccess)
	return nil, output, err
}

func mcpMediaPath(input mcpMediaInput) (string, int, error) {
	if !contains([]string{"", "all", "list", "unwatched", "history", "movies", "shows", "music", "audiobooks", "books", "photos"}, input.View) || !contains([]string{"", "title", "added", "year"}, input.Sort) {
		return "", 0, errors.New("invalid media view or sort")
	}
	queryText := strings.TrimSpace(input.Query)
	if len(queryText) > 512 {
		return "", 0, errors.New("query must not exceed 512 bytes")
	}
	limit := input.Limit
	if limit == 0 {
		limit = 50
	}
	if limit < 1 || limit > 200 {
		return "", 0, errors.New("limit must be between 1 and 200")
	}
	query := url.Values{}
	query.Set("q", queryText)
	query.Set("view", input.View)
	query.Set("sort", input.Sort)
	query.Set("limit", strconv.Itoa(limit))
	return "/api/v1/library?" + query.Encode(), limit, nil
}

func limitMCPItems(output mcpAPIOutput, limit int) mcpAPIOutput {
	body, ok := output.Body.(map[string]any)
	if !ok {
		return output
	}
	items, ok := body["items"].([]any)
	if ok && len(items) > limit {
		body["items"] = items[:limit]
	}
	return output
}
