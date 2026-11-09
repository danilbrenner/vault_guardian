package server

import (
	"context"
	"errors"
	"time"

	"bw-mcp/internal/bw"
	"bw-mcp/internal/cache"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func registerTools(s *mcp.Server, store *cache.Cache) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "ping",
		Description: "Check that the Vault Guardian MCP server is running.",
	}, func(context.Context, *mcp.CallToolRequest, struct{}) (*mcp.CallToolResult, pingResult, error) {
		return nil, pingResult{Status: "ok"}, nil
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "list_items",
		Description: "List cached vault items. Returns id, name, folder, and age in days. Refreshes once if the cache has never been synced.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, []bw.Item, error) {
		if store == nil {
			return nil, nil, errors.New("vault cache is not configured")
		}
		items, err := store.ListItems(ctx)
		if err != nil {
			return nil, nil, err
		}
		if items == nil {
			items = []bw.Item{}
		}
		return nil, items, nil
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "get_item",
		Description: "Get one cached vault item by id. Does not sync.",
	}, func(_ context.Context, _ *mcp.CallToolRequest, in getItemInput) (*mcp.CallToolResult, bw.Item, error) {
		if store == nil {
			return nil, bw.Item{}, errors.New("vault cache is not configured")
		}
		if in.ID == "" {
			return nil, bw.Item{}, errors.New("item id is required")
		}
		item, err := store.GetItem(in.ID)
		if err != nil {
			return nil, bw.Item{}, err
		}
		return nil, item, nil
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "force_sync",
		Description: "Refresh the vault cache. Returns the sync time and item count.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, syncResult, error) {
		if store == nil {
			return nil, syncResult{}, errors.New("vault cache is not configured")
		}
		syncedAt, n, err := store.ForceSync(ctx)
		if err != nil {
			return nil, syncResult{}, err
		}
		return nil, syncResult{
			SyncedAt:   syncedAt.UTC().Format(time.RFC3339),
			ItemsCount: n,
		}, nil
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "cache_status",
		Description: "Report cache age and Bitwarden CLI status. Does not sync.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, cacheStatusResult, error) {
		if store == nil {
			return nil, cacheStatusResult{}, errors.New("vault cache is not configured")
		}
		st, err := store.Status(ctx)
		if err != nil {
			return nil, cacheStatusResult{}, err
		}
		out := cacheStatusResult{
			CacheAgeMinutes: st.AgeMinutes,
			ItemsInCache:    st.Items,
			BWStatus:        st.BWStatus,
		}
		if !st.LastSync.IsZero() {
			formatted := st.LastSync.UTC().Format(time.RFC3339)
			out.LastSync = &formatted
		}
		return nil, out, nil
	})
}

type pingResult struct {
	Status string `json:"status" jsonschema:"ok when the server is running"`
}

type getItemInput struct {
	ID string `json:"id" jsonschema:"Bitwarden item id"`
}

type syncResult struct {
	SyncedAt   string `json:"synced_at" jsonschema:"ISO 8601 time of the sync"`
	ItemsCount int    `json:"items_count" jsonschema:"number of items now in the cache"`
}

type cacheStatusResult struct {
	LastSync        *string `json:"last_sync" jsonschema:"ISO 8601 time of the last sync, or null if the cache has never synced"`
	CacheAgeMinutes int     `json:"cache_age_minutes" jsonschema:"minutes since the last sync"`
	ItemsInCache    int     `json:"items_in_cache" jsonschema:"number of items in the cache"`
	BWStatus        string  `json:"bw_status" jsonschema:"Bitwarden CLI status: unauthenticated, locked, or unlocked"`
}
