package server

import (
	"context"
	"errors"
	"sort"
	"time"

	"bw-mcp/internal/bw"
	"bw-mcp/internal/cache"
	"bw-mcp/internal/metrics"

	"github.com/google/jsonschema-go/jsonschema"
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
		Description: "List cached vault items and password metrics. Refreshes once if the cache has never been synced.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"filter": map[string]any{
					"type": "string",
					"enum": []string{"all", "weak", "duplicate"},
				},
				"min_age_days": map[string]any{"type": "integer"},
			},
			"additionalProperties": false,
		},
		OutputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"items": map[string]any{
					"anyOf": []any{
						map[string]any{"type": "null"},
						map[string]any{
							"type": "array",
							"items": map[string]any{
								"type": "object",
								"properties": map[string]any{
									"id":              map[string]any{"type": "string"},
									"name":            map[string]any{"type": "string"},
									"masked_username": map[string]any{"type": "string"},
									"folder":          map[string]any{"anyOf": []any{map[string]any{"type": "null"}, map[string]any{"type": "string"}}},
									"age_days":        map[string]any{"type": "integer"},
									"length":          map[string]any{"type": "integer"},
									"entropy":         map[string]any{"type": "number"},
									"strength":        map[string]any{"type": "string"},
									"hash":            map[string]any{"type": "string"},
								},
								"required":             []string{"id", "name", "masked_username", "folder", "age_days", "length", "entropy", "strength", "hash"},
								"additionalProperties": false,
							},
						},
					},
				},
			},
			"required":             []string{"items"},
			"additionalProperties": false,
		},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in listItemsInput) (*mcp.CallToolResult, listItemsResult, error) {
		if store == nil {
			return nil, listItemsResult{}, errors.New("vault cache is not configured")
		}
		items, err := store.ListItems(ctx)
		if err != nil {
			return nil, listItemsResult{}, err
		}
		if items == nil {
			items = []bw.Login{}
		}
		items = filterLogins(items, in)
		return nil, listItemsResult{Items: items}, nil
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:         "get_item",
		Description:  "Get one cached vault item by id. Does not sync.",
		OutputSchema: itemOutputSchema(),
	}, func(_ context.Context, _ *mcp.CallToolRequest, in getItemInput) (*mcp.CallToolResult, bw.Login, error) {
		if store == nil {
			return nil, bw.Login{}, errors.New("vault cache is not configured")
		}
		if in.ID == "" {
			return nil, bw.Login{}, errors.New("item id is required")
		}
		item, err := store.GetItem(in.ID)
		if err != nil {
			return nil, bw.Login{}, err
		}
		return nil, item, nil
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "get_duplicates",
		Description: "List groups of logins with duplicate passwords.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"min_count": map[string]any{"type": "integer"},
			},
			"additionalProperties": false,
		},
		OutputSchema: duplicatesOutputSchema(),
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in getDuplicatesInput) (*mcp.CallToolResult, getDuplicatesResult, error) {
		if store == nil {
			return nil, getDuplicatesResult{}, errors.New("vault cache is not configured")
		}
		items, err := store.ListItems(ctx)
		if err != nil {
			return nil, getDuplicatesResult{}, err
		}
		return nil, findDuplicateGroups(items, in.MinCount), nil
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
		Name:         "cache_status",
		Description:  "Report cache age and Bitwarden CLI status. Does not sync.",
		OutputSchema: cacheStatusOutputSchema(),
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

type listItemsInput struct {
	Filter     string `json:"filter,omitempty"`
	MinAgeDays *int   `json:"min_age_days,omitempty"`
}

type getDuplicatesInput struct {
	MinCount *int `json:"min_count,omitempty"`
}

type listItemsResult struct {
	Items []bw.Login `json:"items"`
}

type getDuplicatesResult []duplicateGroup

type duplicateGroup struct {
	Hash   string           `json:"hash"`
	Count  int              `json:"count"`
	Logins []duplicateLogin `json:"logins"`
}

type duplicateLogin struct {
	ID             string  `json:"id"`
	Name           string  `json:"name"`
	MaskedUsername string  `json:"masked_username"`
	Folder         *string `json:"folder"`
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

func filterLogins(items []bw.Login, in listItemsInput) []bw.Login {
	duplicateCounts := make(map[string]int)
	if in.Filter == "duplicate" {
		for _, login := range items {
			duplicateCounts[login.Hash]++
		}
	}

	out := make([]bw.Login, 0, len(items))
	for _, login := range items {
		if in.MinAgeDays != nil && login.AgeDays < *in.MinAgeDays {
			continue
		}
		switch in.Filter {
		case "", "all":
		case "weak":
			if login.Strength != metrics.StrengthVeryWeak && login.Strength != metrics.StrengthWeak {
				continue
			}
		case "duplicate":
			if duplicateCounts[login.Hash] < 2 {
				continue
			}
		default:
			continue
		}
		out = append(out, login)
	}
	return out
}

func findDuplicateGroups(items []bw.Login, minCount *int) getDuplicatesResult {
	threshold := 2
	if minCount != nil {
		threshold = *minCount
	}

	byHash := make(map[string][]duplicateLogin)
	for _, login := range items {
		byHash[login.Hash] = append(byHash[login.Hash], duplicateLogin{
			ID:             login.ID,
			Name:           login.Name,
			MaskedUsername: login.MaskedUsername,
			Folder:         login.Folder,
		})
	}

	hashes := make([]string, 0, len(byHash))
	for hash, members := range byHash {
		if len(members) >= threshold {
			hashes = append(hashes, hash)
		}
	}
	sort.Strings(hashes)

	groups := make(getDuplicatesResult, 0, len(hashes))
	for _, hash := range hashes {
		members := byHash[hash]
		groups = append(groups, duplicateGroup{
			Hash:   hash,
			Count:  len(members),
			Logins: members,
		})
	}
	return groups
}

func itemOutputSchema() *jsonschema.Schema {
	schema, err := jsonschema.For[bw.Login](nil)
	if err != nil {
		panic(err)
	}

	folder := schema.Properties["folder"]
	folder.Type = ""
	folder.Types = nil
	folder.AnyOf = []*jsonschema.Schema{
		{Type: "null"},
		{Type: "string"},
	}
	return schema
}

func duplicatesOutputSchema() map[string]any {
	return map[string]any{
		"type": "array",
		"items": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"hash":  map[string]any{"type": "string"},
				"count": map[string]any{"type": "integer"},
				"logins": map[string]any{
					"type": "array",
					"items": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"id":              map[string]any{"type": "string"},
							"name":            map[string]any{"type": "string"},
							"masked_username": map[string]any{"type": "string"},
							"folder":          map[string]any{"anyOf": []any{map[string]any{"type": "null"}, map[string]any{"type": "string"}}},
						},
						"required":             []string{"id", "name", "masked_username", "folder"},
						"additionalProperties": false,
					},
				},
			},
			"required":             []string{"hash", "count", "logins"},
			"additionalProperties": false,
		},
	}
}

func cacheStatusOutputSchema() *jsonschema.Schema {
	schema, err := jsonschema.For[cacheStatusResult](nil)
	if err != nil {
		panic(err)
	}

	lastSync := schema.Properties["last_sync"]
	lastSync.Type = ""
	lastSync.Types = nil
	lastSync.AnyOf = []*jsonschema.Schema{
		{Type: "null"},
		{Type: "string"},
	}
	return schema
}
