package server

import (
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"
	"time"

	"bw-mcp/internal/bw"
	"bw-mcp/internal/cache"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestVaultToolJSONHasNoSecrets(t *testing.T) {
	now := time.Date(2024, 1, 31, 0, 0, 0, 0, time.UTC)
	clock := now
	runner := &toolRunner{
		items: []byte(`[
			{
				"id":"github","name":"GitHub","folderId":"folder-1","type":1,
				"revisionDate":"2024-01-30T00:00:00.000Z",
				"creationDate":"2023-12-01T00:00:00.000Z",
				"notes":"note-secret",
				"fields":[{"name":"pin","value":"field-secret"}],
				"attachments":[{"fileName":"attach-secret"}],
				"passwordHistory":[{"password":"history-secret"}],
				"login":{
					"username":"danil.secret@example.com",
					"password":"p@ss-should-not-leak",
					"totp":"totp-should-not-leak",
					"passwordRevisionDate":"2024-01-01T00:00:00.000Z",
					"uris":[{"uri":"https://secret.example/uri-secret"}]
				}
			},
			{
				"id":"legacy","name":"Legacy","folderId":null,"type":1,
				"revisionDate":"2024-01-30T00:00:00.000Z",
				"creationDate":"2023-12-01T00:00:00.000Z",
				"login":{"username":"legacy-user","password":"legacy-secret","passwordRevisionDate":null}
			}
		]`),
		folders: []byte(`[{"id":"folder-1","name":"Work"}]`),
	}
	cli := &bw.CLI{Runner: runner, DataDir: "/var/lib/bw"}
	store := cache.New(cli, toolSecrets{}, func() time.Time { return clock })
	session := connectServer(t, New(slog.New(slog.DiscardHandler), store))
	ctx := context.Background()

	if init := session.InitializeResult(); init == nil || strings.Contains(init.Instructions, "No vault tools") {
		t.Fatalf("instructions = %#v", init)
	}

	statusRes := callTool(t, session, "cache_status", nil)
	if strings.Join(runner.calls, ",") != "status" {
		t.Fatalf("cache_status calls = %#v", runner.calls)
	}
	var status cacheStatusResult
	unmarshalTool(t, statusRes, &status)
	if status.LastSync != nil || status.CacheAgeMinutes != 0 || status.ItemsInCache != 0 || status.BWStatus != "locked" {
		t.Fatalf("status = %+v", status)
	}

	listRes := callTool(t, session, "list_items", nil)
	var listed struct {
		Items []map[string]any `json:"items"`
	}
	unmarshalTool(t, listRes, &listed)
	items := listed.Items
	if len(items) != 2 {
		t.Fatalf("items = %#v", items)
	}
	if items[0]["id"] != "github" || items[0]["name"] != "GitHub" || items[0]["folder"] != "Work" || items[0]["age_days"] != float64(30) {
		t.Fatalf("github = %#v", items[0])
	}
	if items[1]["folder"] != nil || items[1]["age_days"] != float64(61) {
		t.Fatalf("legacy = %#v", items[1])
	}
	for _, item := range items {
		for k := range item {
			switch k {
			case "id", "name", "folder", "age_days":
			default:
				t.Fatalf("unexpected field %s", k)
			}
		}
	}
	assertNoSecrets(t, toolText(t, listRes))
	syncs := strings.Count(strings.Join(runner.calls, ","), "sync")
	callTool(t, session, "list_items", nil)
	if strings.Count(strings.Join(runner.calls, ","), "sync") != syncs {
		t.Fatalf("second list refreshed: %#v", runner.calls)
	}

	got := callTool(t, session, "get_item", map[string]any{"id": "github"})
	var item map[string]any
	unmarshalTool(t, got, &item)
	if item["id"] != "github" || item["folder"] != "Work" {
		t.Fatalf("get = %#v", item)
	}
	assertNoSecrets(t, toolText(t, got))

	missing, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "get_item",
		Arguments: map[string]any{"id": "missing"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !missing.IsError {
		t.Fatal("expected not-found tool error")
	}
	assertNoSecrets(t, toolText(t, missing))
	if !strings.Contains(toolText(t, missing), "missing") {
		t.Fatalf("not-found text = %s", toolText(t, missing))
	}

	before := len(runner.calls)
	forced := callTool(t, session, "force_sync", nil)
	var synced syncResult
	unmarshalTool(t, forced, &synced)
	if synced.SyncedAt != "2024-01-31T00:00:00Z" || synced.ItemsCount != 2 {
		t.Fatalf("sync = %+v", synced)
	}
	if len(runner.calls) == before {
		t.Fatal("force_sync did not call bw")
	}
	assertNoSecrets(t, toolText(t, forced))

	clock = clock.Add(90 * time.Minute)
	after := callTool(t, session, "cache_status", nil)
	unmarshalTool(t, after, &status)
	if status.LastSync == nil || *status.LastSync != "2024-01-31T00:00:00Z" || status.CacheAgeMinutes != 90 || status.ItemsInCache != 2 || status.BWStatus != "locked" {
		t.Fatalf("status after sync = %+v", status)
	}
	assertNoSecrets(t, toolText(t, after))
}

func TestListItemsErrorOmitsPassword(t *testing.T) {
	runner := &toolRunner{folders: []byte("[]"), failItems: true}
	cli := &bw.CLI{Runner: runner, DataDir: "/var/lib/bw"}
	store := cache.New(cli, toolSecrets{}, time.Now)
	session := connectServer(t, New(slog.New(slog.DiscardHandler), store))
	res, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: "list_items"})
	if err != nil {
		t.Fatal(err)
	}
	if !res.IsError {
		t.Fatal("expected tool error")
	}
	text := toolText(t, res)
	if strings.Contains(text, "p@ss-should-not-leak") || strings.Contains(text, "master-password") {
		t.Fatalf("error leaked: %s", text)
	}
	if !strings.Contains(text, "locked") {
		t.Fatalf("error = %s", text)
	}
}

func TestListItemsOutputSchemaIsObject(t *testing.T) {
	session := connectServer(t, New(slog.New(slog.DiscardHandler), nil))
	tools, err := session.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, tool := range tools.Tools {
		if tool.Name != "list_items" {
			continue
		}
		schema, ok := tool.OutputSchema.(map[string]any)
		if !ok {
			t.Fatalf("list_items output schema has type %T", tool.OutputSchema)
		}
		if schema["type"] != "object" {
			t.Fatalf("list_items output schema type = %v, want object", schema["type"])
		}
		properties := schema["properties"].(map[string]any)
		items := properties["items"].(map[string]any)
		assertNullableSchema(t, items, "list_items.items")
		arraySchema := items["anyOf"].([]any)[1].(map[string]any)
		itemSchema := arraySchema["items"].(map[string]any)
		itemProperties := itemSchema["properties"].(map[string]any)
		assertNullableSchema(t, itemProperties["folder"].(map[string]any), "list_items.items[].folder")
		return
	}
	t.Fatal("list_items tool not found")
}

func TestCacheStatusNullableLastSyncSchemaUsesAnyOf(t *testing.T) {
	session := connectServer(t, New(slog.New(slog.DiscardHandler), nil))
	tools, err := session.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, tool := range tools.Tools {
		if tool.Name != "cache_status" {
			continue
		}
		schema, ok := tool.OutputSchema.(map[string]any)
		if !ok {
			t.Fatalf("cache_status output schema has type %T", tool.OutputSchema)
		}
		properties := schema["properties"].(map[string]any)
		lastSync := properties["last_sync"].(map[string]any)
		assertNullableSchema(t, lastSync, "cache_status.last_sync")
		branches := lastSync["anyOf"].([]any)
		if branches[1].(map[string]any)["type"] != "string" {
			t.Fatalf("cache_status.last_sync non-null branch = %#v, want string", branches[1])
		}
		required, ok := schema["required"].([]any)
		if !ok {
			t.Fatalf("cache_status required fields have type %T", schema["required"])
		}
		for _, field := range required {
			if field == "last_sync" {
				return
			}
		}
		t.Fatal("cache_status.last_sync must remain required and nullable")
	}
	t.Fatal("cache_status tool not found")
}

func TestGetItemNullableFolderSchemaUsesAnyOf(t *testing.T) {
	session := connectServer(t, New(slog.New(slog.DiscardHandler), nil))
	tools, err := session.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, tool := range tools.Tools {
		if tool.Name != "get_item" {
			continue
		}
		schema, ok := tool.OutputSchema.(map[string]any)
		if !ok {
			t.Fatalf("get_item output schema has type %T", tool.OutputSchema)
		}
		properties := schema["properties"].(map[string]any)
		folder := properties["folder"].(map[string]any)
		assertNullableSchema(t, folder, "get_item.folder")
		branches := folder["anyOf"].([]any)
		if branches[1].(map[string]any)["type"] != "string" {
			t.Fatalf("get_item.folder non-null branch = %#v, want string", branches[1])
		}
		required, ok := schema["required"].([]any)
		if !ok {
			t.Fatalf("get_item required fields have type %T", schema["required"])
		}
		for _, field := range required {
			if field == "folder" {
				return
			}
		}
		t.Fatal("get_item.folder must remain required and nullable")
	}
	t.Fatal("get_item tool not found")
}

func assertNullableSchema(t *testing.T, schema map[string]any, path string) {
	t.Helper()
	if _, hasType := schema["type"]; hasType {
		t.Fatalf("%s should use anyOf branches, got type %v", path, schema["type"])
	}
	branches, ok := schema["anyOf"].([]any)
	if !ok || len(branches) != 2 {
		t.Fatalf("%s anyOf = %#v, want null and non-null branches", path, schema["anyOf"])
	}
	first, ok := branches[0].(map[string]any)
	if !ok || first["type"] != "null" {
		t.Fatalf("%s null branch = %#v", path, branches[0])
	}
	second, ok := branches[1].(map[string]any)
	if !ok || second["type"] != "array" && second["type"] != "string" {
		t.Fatalf("%s non-null branch = %#v", path, branches[1])
	}
}

type toolSecrets struct{}

func (toolSecrets) LoadClientID() ([]byte, error) {
	return []byte("client-id"), nil
}

func (toolSecrets) LoadClientSecret() ([]byte, error) {
	return []byte("client-secret"), nil
}

func (toolSecrets) LoadMasterPassword() ([]byte, error) {
	return []byte("master-password"), nil
}

func connectServer(t *testing.T, s *mcp.Server) *mcp.ClientSession {
	t.Helper()
	ctx := context.Background()
	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	if _, err := s.Connect(ctx, serverTransport, nil); err != nil {
		t.Fatal(err)
	}
	client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "v0.0.1"}, nil)
	session, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = session.Close() })
	return session
}

func callTool(t *testing.T, session *mcp.ClientSession, name string, args any) *mcp.CallToolResult {
	t.Helper()
	res, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	if res.IsError {
		t.Fatalf("%s tool error: %s", name, toolText(t, res))
	}
	return res
}

func unmarshalTool(t *testing.T, res *mcp.CallToolResult, dest any) {
	t.Helper()
	b, err := json.Marshal(res.StructuredContent)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(b, dest); err != nil {
		t.Fatalf("unmarshal %s: %v", b, err)
	}
}

func toolText(t *testing.T, res *mcp.CallToolResult) string {
	t.Helper()
	var b strings.Builder
	if res.StructuredContent != nil {
		encoded, err := json.Marshal(res.StructuredContent)
		if err != nil {
			t.Fatal(err)
		}
		b.Write(encoded)
	}
	for _, c := range res.Content {
		if text, ok := c.(*mcp.TextContent); ok {
			b.WriteString(text.Text)
		}
	}
	return b.String()
}

func assertNoSecrets(t *testing.T, got string) {
	t.Helper()
	for _, secret := range []string{
		"p@ss-should-not-leak",
		"danil.secret@example.com",
		"totp-should-not-leak",
		"note-secret",
		"field-secret",
		"attach-secret",
		"history-secret",
		"uri-secret",
		"legacy-secret",
		"legacy-user",
		"master-password",
		"client-secret",
		"sess-from-unlock",
		`"password"`,
		`"username"`,
		`"totp"`,
		`"pwned_count"`,
		`"entropy"`,
		`"strength"`,
		`"hash"`,
	} {
		if strings.Contains(got, secret) {
			t.Fatalf("tool JSON contains %q: %s", secret, got)
		}
	}
}

type toolRunner struct {
	items     []byte
	folders   []byte
	failItems bool
	calls     []string
}

func (r *toolRunner) Run(ctx context.Context, args []string, env []string) (bw.Result, error) {
	cmd := strings.Join(args, " ")
	r.calls = append(r.calls, cmd)
	if strings.Contains(cmd, "master-password") || strings.Contains(cmd, "sess-from-unlock") || strings.Contains(cmd, "client-secret") {
		return bw.Result{ExitCode: 1, Stderr: []byte("secret in argv")}, &bw.ExitError{ExitCode: 1}
	}
	switch cmd {
	case "status":
		return bw.Result{Stdout: []byte(`{"status":"locked","userId":"user-1","serverUrl":"https://bw.example"}`)}, nil
	case "unlock --passwordenv BW_PASSWORD --raw":
		return bw.Result{Stdout: []byte("sess-from-unlock\n")}, nil
	case "sync", "lock":
		return bw.Result{}, nil
	case "list folders":
		return bw.Result{Stdout: r.folders}, nil
	case "list items":
		if r.failItems {
			return bw.Result{Stdout: []byte("p@ss-should-not-leak"), Stderr: []byte("locked"), ExitCode: 1}, &bw.ExitError{ExitCode: 1}
		}
		return bw.Result{Stdout: r.items}, nil
	default:
		return bw.Result{ExitCode: 1, Stderr: []byte("unexpected " + cmd)}, &bw.ExitError{ExitCode: 1}
	}
}
