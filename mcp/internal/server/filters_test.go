package server

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"slices"
	"strings"
	"testing"
	"time"

	"bw-mcp/internal/bw"
	"bw-mcp/internal/cache"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestListItemsFiltersWeakAndMinAge(t *testing.T) {
	now := time.Date(2024, 1, 31, 0, 0, 0, 0, time.UTC)
	session, runner := filterTestSession(t, now)

	all := listItemsForTest(t, session, nil)
	assertLoginIDs(t, all, "dup-old-1", "dup-young", "dup-old-2", "very-weak", "strong", "weak-young", "dup-strong-1", "dup-strong-2")

	weak := listItemsForTest(t, session, map[string]any{"filter": "weak"})
	assertLoginIDs(t, weak, "dup-old-1", "dup-young", "dup-old-2", "very-weak", "weak-young")
	for _, login := range weak {
		if strength := login["strength"]; strength != "VeryWeak" && strength != "Weak" {
			t.Errorf("weak filter returned strength %v for id %v", strength, login["id"])
		}
	}

	old := listItemsForTest(t, session, map[string]any{"min_age_days": 50})
	assertLoginIDs(t, old, "dup-old-1", "dup-old-2", "strong", "dup-strong-1")

	weakAndOld := listItemsForTest(t, session, map[string]any{"filter": "weak", "min_age_days": 50})
	assertLoginIDs(t, weakAndOld, "dup-old-1", "dup-old-2")

	duplicate := listItemsForTest(t, session, map[string]any{"filter": "duplicate"})
	assertLoginIDs(t, duplicate, "dup-old-1", "dup-young", "dup-old-2", "dup-strong-1", "dup-strong-2")

	duplicateAndOld := listItemsForTest(t, session, map[string]any{"filter": "duplicate", "min_age_days": 50})
	assertLoginIDs(t, duplicateAndOld, "dup-old-1", "dup-old-2", "dup-strong-1")

	if got := countToolCalls(runner.calls, "sync"); got != 1 {
		t.Errorf("filter requests caused %d syncs, want only the initial cache fill", got)
	}
}

func TestGetDuplicatesDefaultAndMinCount(t *testing.T) {
	now := time.Date(2024, 1, 31, 0, 0, 0, 0, time.UTC)
	session, runner := filterTestSession(t, now)
	_ = listItemsForTest(t, session, nil) // Populate the cache once; duplicate lookup must not force another sync.
	syncs := countToolCalls(runner.calls, "sync")

	defaultGroups := getDuplicatesForTest(t, session, nil)
	if len(defaultGroups) != 2 {
		t.Fatalf("default duplicate groups = %d, want 2", len(defaultGroups))
	}
	assertDuplicateGroups(t, defaultGroups)

	threeOrMore := getDuplicatesForTest(t, session, map[string]any{"min_count": 3})
	if len(threeOrMore) != 1 {
		t.Fatalf("min_count=3 groups = %d, want 1", len(threeOrMore))
	}
	assertDuplicateGroup(t, threeOrMore[0], duplicateHashForTest("aA1!bcDE"), 3, []string{"dup-old-1", "dup-young", "dup-old-2"})

	fourOrMore := getDuplicatesForTest(t, session, map[string]any{"min_count": 4})
	if len(fourOrMore) != 0 {
		t.Fatalf("min_count=4 groups = %#v, want none", fourOrMore)
	}
	if got := countToolCalls(runner.calls, "sync"); got != syncs {
		t.Errorf("get_duplicates caused extra syncs: before=%d after=%d", syncs, got)
	}
}

func filterTestSession(t *testing.T, now time.Time) (*mcp.ClientSession, *toolRunner) {
	t.Helper()
	items := []filterFixture{
		{id: "dup-old-1", name: "Duplicate old one", username: "danil@gmail.com", password: "aA1!bcDE", ageDays: 100, folderID: "work"},
		{id: "dup-young", name: "Duplicate young", username: "bobbie@gmail.com", password: "aA1!bcDE", ageDays: 5},
		{id: "dup-old-2", name: "Duplicate old two", username: "charlie@gmail.com", password: "aA1!bcDE", ageDays: 200},
		{id: "very-weak", name: "Very weak", username: "vicky@gmail.com", password: "abc", ageDays: 20},
		{id: "strong", name: "Strong unique", username: "sally@gmail.com", password: "aA1!bcDElongX", ageDays: 300},
		{id: "weak-young", name: "Weak unique", username: "wendy@gmail.com", password: "xY2!zT7q", ageDays: 1},
		{id: "dup-strong-1", name: "Strong duplicate one", username: "penny@gmail.com", password: "B2$xyZ9Fabcde", ageDays: 50, folderID: "personal"},
		{id: "dup-strong-2", name: "Strong duplicate two", username: "robin@gmail.com", password: "B2$xyZ9Fabcde", ageDays: 25},
	}
	itemBody, err := json.Marshal(filterFixturesJSON(now, items))
	if err != nil {
		t.Fatal(err)
	}
	runner := &toolRunner{
		items:   itemBody,
		folders: []byte(`[{"id":"work","name":"Work"},{"id":"personal","name":"Personal"}]`),
	}
	cli := &bw.CLI{Runner: runner, DataDir: "/var/lib/bw"}
	store := cache.New(cli, toolSecrets{}, func() time.Time { return now })
	return connectServer(t, New(slog.New(slog.DiscardHandler), store)), runner
}

type filterFixture struct {
	id       string
	name     string
	username string
	password string
	ageDays  int
	folderID string
}

func filterFixturesJSON(now time.Time, fixtures []filterFixture) []map[string]any {
	items := make([]map[string]any, 0, len(fixtures))
	for _, fixture := range fixtures {
		changed := now.Add(-time.Duration(fixture.ageDays) * 24 * time.Hour).Format(time.RFC3339)
		var folderID any
		if fixture.folderID != "" {
			folderID = fixture.folderID
		}
		items = append(items, map[string]any{
			"id":           fixture.id,
			"name":         fixture.name,
			"folderId":     folderID,
			"type":         1,
			"creationDate": changed,
			"login": map[string]any{
				"username":             fixture.username,
				"password":             fixture.password,
				"passwordRevisionDate": changed,
			},
		})
	}
	return items
}

func listItemsForTest(t *testing.T, session *mcp.ClientSession, args any) []map[string]any {
	t.Helper()
	res := callTool(t, session, "list_items", args)
	var result struct {
		Items []map[string]any `json:"items"`
	}
	unmarshalTool(t, res, &result)
	return result.Items
}

func getDuplicatesForTest(t *testing.T, session *mcp.ClientSession, args any) []map[string]any {
	t.Helper()
	res := callTool(t, session, "get_duplicates", args)
	var groups []map[string]any
	unmarshalTool(t, res, &groups)
	return groups
}

func assertLoginIDs(t *testing.T, logins []map[string]any, want ...string) {
	t.Helper()
	got := make([]string, 0, len(logins))
	for _, login := range logins {
		id, ok := login["id"].(string)
		if !ok {
			t.Errorf("login id has type %T", login["id"])
			continue
		}
		got = append(got, id)
	}
	slices.Sort(got)
	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Errorf("login ids = %v, want %v", got, want)
	}
}

func assertDuplicateGroups(t *testing.T, groups []map[string]any) {
	t.Helper()
	wantHashes := map[string]struct {
		count int
		ids   []string
	}{
		duplicateHashForTest("aA1!bcDE"):      {count: 3, ids: []string{"dup-old-1", "dup-young", "dup-old-2"}},
		duplicateHashForTest("B2$xyZ9Fabcde"): {count: 2, ids: []string{"dup-strong-1", "dup-strong-2"}},
	}
	if len(groups) != len(wantHashes) {
		t.Errorf("duplicate groups = %d, want %d", len(groups), len(wantHashes))
	}
	for _, group := range groups {
		hash, _ := group["hash"].(string)
		want, ok := wantHashes[hash]
		if !ok {
			t.Errorf("unexpected duplicate group hash %q", hash)
			continue
		}
		assertDuplicateGroup(t, group, hash, want.count, want.ids)
	}
}

func assertDuplicateGroup(t *testing.T, group map[string]any, hash string, count int, wantIDs []string) {
	t.Helper()
	if group["hash"] != hash || group["count"] != float64(count) {
		t.Errorf("duplicate group header = hash %v count %v, want %s %d", group["hash"], group["count"], hash, count)
	}
	logins, ok := group["logins"].([]any)
	if !ok {
		t.Errorf("duplicate group logins has type %T", group["logins"])
		return
	}
	gotIDs := make([]string, 0, len(logins))
	for _, raw := range logins {
		login, ok := raw.(map[string]any)
		if !ok {
			t.Errorf("duplicate member has type %T", raw)
			continue
		}
		id, _ := login["id"].(string)
		gotIDs = append(gotIDs, id)
		if login["name"] == nil || login["masked_username"] == nil || login["folder"] == nil && !hasNullFolder(login) {
			t.Errorf("duplicate member is missing public summary fields: %#v", login)
		}
		keys := make([]string, 0, len(login))
		for key := range login {
			keys = append(keys, key)
		}
		slices.Sort(keys)
		if !slices.Equal(keys, []string{"folder", "id", "masked_username", "name"}) {
			t.Errorf("duplicate member fields = %v, want id, name, masked_username, folder", keys)
		}
	}
	slices.Sort(gotIDs)
	slices.Sort(wantIDs)
	if !slices.Equal(gotIDs, wantIDs) {
		t.Errorf("duplicate member ids = %v, want %v", gotIDs, wantIDs)
	}
}

func hasNullFolder(login map[string]any) bool {
	value, exists := login["folder"]
	return exists && value == nil
}

func duplicateHashForTest(password string) string {
	digest := sha256.Sum256([]byte(password + "user-1"))
	return hex.EncodeToString(digest[:])
}

func countToolCalls(calls []string, name string) int {
	count := 0
	for _, call := range calls {
		if strings.TrimSpace(call) == name {
			count++
		}
	}
	return count
}
