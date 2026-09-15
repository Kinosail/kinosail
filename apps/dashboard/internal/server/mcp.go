package server

import (
	"context"
	"errors"
	"net/http"

	"github.com/MikeO7/kinosail-dashboard/internal/dashboard"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type mcpListInput struct{}

type mcpAppInput struct {
	CatalogID       string `json:"catalogId,omitempty" jsonschema:"Built-in catalog ID such as jellyfin, plex, sonarr, home-assistant, or proxmox"`
	Name            string `json:"name,omitempty" jsonschema:"Application name; omit when catalogId supplies it"`
	URL             string `json:"url" jsonschema:"Direct HTTP or HTTPS address opened by the household browser"`
	HealthURL       string `json:"healthUrl,omitempty" jsonschema:"Optional HTTP or HTTPS address used for a bounded reachability check"`
	CheckEnabled    bool   `json:"checkEnabled" jsonschema:"Explicitly allow periodic Server reachability checks for this address"`
	Description     string `json:"description,omitempty" jsonschema:"Short household-facing description"`
	Category        string `json:"category,omitempty" jsonschema:"Short board category"`
	Icon            string `json:"icon,omitempty" jsonschema:"Local icon token; omit when catalogId supplies it"`
	Accent          string `json:"accent,omitempty" jsonschema:"One of lime, ocean, amber, coral, sky, violet, or slate"`
	Favorite        bool   `json:"favorite,omitempty" jsonschema:"Whether to mark the application as a favorite"`
	ExpectedVersion uint64 `json:"expectedVersion" jsonschema:"Required board version from list_applications; rejects silent concurrent changes"`
}

type mcpUpdateInput struct {
	ID              string  `json:"id" jsonschema:"Application ID from list_applications"`
	Name            *string `json:"name,omitempty"`
	URL             *string `json:"url,omitempty"`
	HealthURL       *string `json:"healthUrl,omitempty"`
	CheckEnabled    *bool   `json:"checkEnabled,omitempty"`
	Description     *string `json:"description,omitempty"`
	Category        *string `json:"category,omitempty"`
	Accent          *string `json:"accent,omitempty"`
	Favorite        *bool   `json:"favorite,omitempty"`
	ExpectedVersion uint64  `json:"expectedVersion" jsonschema:"Required board version from list_applications"`
}

type mcpRemoveInput struct {
	ID              string `json:"id" jsonschema:"Application ID from list_applications"`
	ExpectedVersion uint64 `json:"expectedVersion" jsonschema:"Required board version from list_applications"`
	Confirm         bool   `json:"confirm" jsonschema:"Must be true after the user confirms removal"`
}

type mcpOrderInput struct {
	IDs             []string `json:"ids" jsonschema:"Every application ID exactly once in the requested order"`
	ExpectedVersion uint64   `json:"expectedVersion" jsonschema:"Required board version from list_applications"`
}

type mcpCheckInput struct {
	ID string `json:"id" jsonschema:"Application ID to check"`
}

type mcpBoardOutput struct {
	Board dashboard.Snapshot `json:"board"`
}
type mcpCatalogOutput struct {
	Apps []dashboard.CatalogEntry `json:"apps"`
}
type mcpMutationOutput struct {
	Receipt dashboard.Receipt  `json:"receipt"`
	Board   dashboard.Snapshot `json:"board"`
}
type mcpCheckOutput struct {
	Health dashboard.Health `json:"health"`
}

func newMCPServer(board *dashboard.Service, prober *dashboard.Prober) *mcp.Server {
	implementation := &mcp.Implementation{Name: "Kinosail Dashboard", Version: "1"}
	server := mcp.NewServer(implementation, &mcp.ServerOptions{Instructions: "Manage the one responsive Kinosail Dashboard board. Read the current version before mutations. Prefer catalog defaults. Never put credentials in URLs. Removal requires explicit confirmation. Tools return receipts and the resulting board."})
	closed, destructive := false, true
	mcp.AddTool(server, &mcp.Tool{Name: "list_applications", Title: "List dashboard applications", Description: "Return the canonical board order, current version, local destinations, and latest reachability observations.", Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true, IdempotentHint: true, OpenWorldHint: &closed}}, func(context.Context, *mcp.CallToolRequest, mcpListInput) (*mcp.CallToolResult, mcpBoardOutput, error) {
		return nil, mcpBoardOutput{Board: board.Snapshot()}, nil
	})
	mcp.AddTool(server, &mcp.Tool{Name: "list_application_catalog", Title: "List application catalog", Description: "List built-in names, categories, local icon tokens, and accent defaults.", Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true, IdempotentHint: true, OpenWorldHint: &closed}}, func(context.Context, *mcp.CallToolRequest, mcpListInput) (*mcp.CallToolResult, mcpCatalogOutput, error) {
		return nil, mcpCatalogOutput{Apps: dashboard.Catalog()}, nil
	})
	mcp.AddTool(server, &mcp.Tool{Name: "add_application", Title: "Add dashboard application", Description: "Add a direct application destination using catalog defaults where possible. Returns a revision receipt.", Annotations: &mcp.ToolAnnotations{OpenWorldHint: &closed}}, func(callContext context.Context, _ *mcp.CallToolRequest, input mcpAppInput) (*mcp.CallToolResult, mcpMutationOutput, error) {
		_, receipt, err := board.Create(callContext, dashboard.CreateInput(input), "Host MCP")
		return nil, mcpMutationOutput{Receipt: receipt, Board: board.Snapshot()}, err
	})
	mcp.AddTool(server, &mcp.Tool{Name: "update_application", Title: "Update dashboard application", Description: "Update selected fields on one application. Returns a revision receipt.", Annotations: &mcp.ToolAnnotations{OpenWorldHint: &closed}}, func(callContext context.Context, _ *mcp.CallToolRequest, input mcpUpdateInput) (*mcp.CallToolResult, mcpMutationOutput, error) {
		_, receipt, err := board.Update(callContext, input.ID, dashboard.UpdateInput{Name: input.Name, URL: input.URL, HealthURL: input.HealthURL, CheckEnabled: input.CheckEnabled, Description: input.Description, Category: input.Category, Accent: input.Accent, Favorite: input.Favorite, ExpectedVersion: input.ExpectedVersion}, "Host MCP")
		return nil, mcpMutationOutput{Receipt: receipt, Board: board.Snapshot()}, err
	})
	mcp.AddTool(server, &mcp.Tool{Name: "remove_application", Title: "Remove dashboard application", Description: "Remove one application from the board after explicit confirmation. The application remains recoverable.", Annotations: &mcp.ToolAnnotations{DestructiveHint: &destructive, OpenWorldHint: &closed}}, func(callContext context.Context, _ *mcp.CallToolRequest, input mcpRemoveInput) (*mcp.CallToolResult, mcpMutationOutput, error) {
		if !input.Confirm {
			return nil, mcpMutationOutput{}, errors.New("confirm must be true before removal")
		}
		receipt, err := board.Remove(callContext, input.ID, input.ExpectedVersion, "Host MCP")
		return nil, mcpMutationOutput{Receipt: receipt, Board: board.Snapshot()}, err
	})
	mcp.AddTool(server, &mcp.Tool{Name: "reorder_applications", Title: "Reorder dashboard applications", Description: "Atomically set the canonical order using every current application ID exactly once.", Annotations: &mcp.ToolAnnotations{OpenWorldHint: &closed}}, func(callContext context.Context, _ *mcp.CallToolRequest, input mcpOrderInput) (*mcp.CallToolResult, mcpMutationOutput, error) {
		receipt, err := board.Reorder(callContext, input.IDs, input.ExpectedVersion, "Host MCP")
		return nil, mcpMutationOutput{Receipt: receipt, Board: board.Snapshot()}, err
	})
	open := true
	mcp.AddTool(server, &mcp.Tool{Name: "check_application", Title: "Check application reachability", Description: "Perform one bounded server-side reachability check without proxying application content.", Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true, OpenWorldHint: &open}}, func(callContext context.Context, _ *mcp.CallToolRequest, input mcpCheckInput) (*mcp.CallToolResult, mcpCheckOutput, error) {
		health, err := prober.ProbeOne(callContext, input.ID)
		return nil, mcpCheckOutput{Health: health}, err
	})
	return server
}

// NewMCPHandler binds tools to the running Server's live board state.
func NewMCPHandler(board *dashboard.Service, prober *dashboard.Prober) http.Handler {
	return mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server {
		return newMCPServer(board, prober)
	}, &mcp.StreamableHTTPOptions{Stateless: true, JSONResponse: true, MaxRequestBodyBytes: 1 << 20})
}
