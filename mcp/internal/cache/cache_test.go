package cache

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"bw-mcp/internal/bw"
	"bw-mcp/internal/secrets"
)

func TestListItemsRefreshesEmptyCacheOnce(t *testing.T) {
	now := time.Date(2024, 1, 31, 0, 0, 0, 0, time.UTC)
	runner := newScriptRunner(sampleItems())
	store := New(newCLI(runner), testSecrets(), func() time.Time { return now })

	first, err := store.ListItems(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	second, err := store.ListItems(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(first) != 2 || len(second) != 2 {
		t.Fatalf("items = %d then %d", len(first), len(second))
	}
	if first[0].ID != "github" || first[0].AgeDays != 30 || first[0].Folder == nil || *first[0].Folder != "Work" {
		t.Fatalf("first = %+v", first[0])
	}
	if first[1].Folder != nil || first[1].AgeDays != 61 {
		t.Fatalf("second item = %+v", first[1])
	}
	if strings.Count(strings.Join(runner.calls, ","), "sync") != 1 {
		t.Fatalf("calls = %#v", runner.calls)
	}
	assertNoSecrets(t, mustJSON(t, first))
}

func TestEmptyVaultDoesNotRefreshAgain(t *testing.T) {
	runner := newScriptRunner([]byte("[]"))
	runner.folders = []byte("[]")
	store := New(newCLI(runner), testSecrets(), time.Now)
	items, err := store.ListItems(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 0 {
		t.Fatalf("items = %+v", items)
	}
	if _, err := store.ListItems(context.Background()); err != nil {
		t.Fatal(err)
	}
	if strings.Count(strings.Join(runner.calls, ","), "list items") != 1 {
		t.Fatalf("calls = %#v", runner.calls)
	}
}

func TestGetItemDoesNotSync(t *testing.T) {
	runner := newScriptRunner(sampleItems())
	store := New(newCLI(runner), testSecrets(), time.Now)
	_, err := store.GetItem("github")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("error = %v", err)
	}
	if len(runner.calls) != 0 {
		t.Fatalf("calls = %#v", runner.calls)
	}
	if _, err := store.ListItems(context.Background()); err != nil {
		t.Fatal(err)
	}
	item, err := store.GetItem("github")
	if err != nil {
		t.Fatal(err)
	}
	if item.Name != "GitHub" {
		t.Fatalf("item = %+v", item)
	}
	_, err = store.GetItem("missing")
	if !errors.Is(err, ErrNotFound) || strings.Contains(err.Error(), "p@ss-should-not-leak") {
		t.Fatalf("error = %v", err)
	}
}

func TestForceSyncReplacesItems(t *testing.T) {
	now := time.Date(2024, 1, 31, 0, 0, 0, 0, time.UTC)
	runner := newScriptRunner(sampleItems())
	store := New(newCLI(runner), testSecrets(), func() time.Time { return now })
	syncedAt, n, err := store.ForceSync(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !syncedAt.Equal(now) || n != 2 {
		t.Fatalf("syncedAt = %s n = %d", syncedAt, n)
	}
	runner.items = []byte(`[{
		"id":"aws","name":"AWS","type":1,
		"creationDate":"2024-01-21T00:00:00.000Z",
		"login":{"password":"aws-secret","passwordRevisionDate":"2024-01-21T00:00:00.000Z"}
	}]`)
	syncedAt, n, err = store.ForceSync(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("n = %d", n)
	}
	items, err := store.ListItems(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].ID != "aws" || items[0].AgeDays != 10 {
		t.Fatalf("items = %+v", items)
	}
	assertNoSecrets(t, mustJSON(t, items))
}

func TestFailedSyncDoesNotPublish(t *testing.T) {
	runner := newScriptRunner(sampleItems())
	runner.failLock = true
	store := New(newCLI(runner), testSecrets(), time.Now)
	_, _, err := store.ForceSync(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
	if strings.Contains(err.Error(), "p@ss-should-not-leak") || strings.Contains(err.Error(), "master-password") {
		t.Fatalf("error leaked: %v", err)
	}
	st, err := store.Status(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if st.Items != 0 || !st.LastSync.IsZero() || st.BWStatus != bw.StatusLocked {
		t.Fatalf("status = %+v", st)
	}
	if _, err := store.GetItem("github"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("get = %v", err)
	}
}

func TestFailedForceSyncKeepsPreviousItems(t *testing.T) {
	runner := newScriptRunner(sampleItems())
	store := New(newCLI(runner), testSecrets(), time.Now)
	if _, _, err := store.ForceSync(context.Background()); err != nil {
		t.Fatal(err)
	}
	runner.failLock = true
	if _, _, err := store.ForceSync(context.Background()); err == nil {
		t.Fatal("expected error")
	}
	items, err := store.ListItems(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 || items[0].ID != "github" {
		t.Fatalf("items = %+v", items)
	}
}

func TestStatusDoesNotSync(t *testing.T) {
	now := time.Date(2024, 1, 31, 0, 0, 0, 0, time.UTC)
	clock := now
	runner := newScriptRunner(sampleItems())
	store := New(newCLI(runner), testSecrets(), func() time.Time { return clock })

	st, err := store.Status(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if st.BWStatus != bw.StatusLocked || st.Items != 0 || st.AgeMinutes != 0 || !st.LastSync.IsZero() {
		t.Fatalf("status = %+v", st)
	}
	if strings.Join(runner.calls, ",") != "status" {
		t.Fatalf("calls = %#v", runner.calls)
	}

	if _, _, err := store.ForceSync(context.Background()); err != nil {
		t.Fatal(err)
	}
	before := len(runner.calls)
	clock = clock.Add(90*time.Minute + 59*time.Second)
	st, err = store.Status(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if st.Items != 2 || st.AgeMinutes != 90 || !st.LastSync.Equal(now) || st.BWStatus != bw.StatusLocked {
		t.Fatalf("status = %+v", st)
	}
	if strings.Join(runner.calls[before:], ",") != "status" {
		t.Fatalf("calls = %#v", runner.calls)
	}
}

func TestStatusReportsUnauthenticated(t *testing.T) {
	runner := newScriptRunner(nil)
	runner.status = bw.StatusUnauthenticated
	runner.userID = ""
	store := New(newCLI(runner), testSecrets(), time.Now)
	st, err := store.Status(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if st.BWStatus != bw.StatusUnauthenticated {
		t.Fatalf("status = %+v", st)
	}
	if strings.Join(runner.calls, ",") != "status" {
		t.Fatalf("calls = %#v", runner.calls)
	}
}

func TestForceSyncCanceled(t *testing.T) {
	runner := newScriptRunner(sampleItems())
	store := New(newCLI(runner), testSecrets(), time.Now)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, _, err := store.ForceSync(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v", err)
	}
	if len(runner.calls) != 0 {
		t.Fatalf("calls = %#v", runner.calls)
	}
}

func TestListItemsRetriesAfterFailure(t *testing.T) {
	runner := newScriptRunner(sampleItems())
	runner.failLock = true
	store := New(newCLI(runner), testSecrets(), time.Now)
	if _, err := store.ListItems(context.Background()); err == nil {
		t.Fatal("expected error")
	}
	runner.failLock = false
	items, err := store.ListItems(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 {
		t.Fatalf("items = %+v", items)
	}
}

func TestForceSyncOpensAndCloses(t *testing.T) {
	runner := newScriptRunner(sampleItems())
	store := New(newCLI(runner), testSecrets(), time.Now)
	if _, _, err := store.ForceSync(context.Background()); err != nil {
		t.Fatal(err)
	}
	want := "status,unlock --passwordenv BW_PASSWORD --raw,sync,list folders,list items,lock"
	if strings.Join(runner.calls, ",") != want {
		t.Fatalf("calls = %#v", runner.calls)
	}
}

func TestForceSyncLockFailureRetriesWithoutLogout(t *testing.T) {
	runner := newScriptRunner(sampleItems())
	runner.failLock = true
	store := New(newCLI(runner), testSecrets(), time.Now)
	_, _, err := store.ForceSync(context.Background())
	if err == nil || !strings.Contains(err.Error(), "lock failed") {
		t.Fatalf("error = %v", err)
	}
	var unlocks, locks int
	for _, call := range runner.calls {
		switch call {
		case "unlock --passwordenv BW_PASSWORD --raw":
			unlocks++
		case "lock":
			locks++
		}
	}
	if unlocks != 2 || locks != 2 {
		t.Fatalf("unlocks=%d locks=%d calls=%#v", unlocks, locks, runner.calls)
	}
	if strings.Contains(strings.Join(runner.calls, ","), "logout") {
		t.Fatalf("unexpected logout: %#v", runner.calls)
	}
}

func newCLI(runner *scriptRunner) *bw.CLI {
	return &bw.CLI{Runner: runner, DataDir: "/var/lib/bw"}
}

func testSecrets() secrets.Source {
	return testSecretSource{}
}

type testSecretSource struct{}

func (testSecretSource) LoadClientID() ([]byte, error) {
	return []byte("client-id"), nil
}

func (testSecretSource) LoadClientSecret() ([]byte, error) {
	return []byte("client-secret"), nil
}

func (testSecretSource) LoadMasterPassword() ([]byte, error) {
	return []byte("master-password"), nil
}

func sampleItems() []byte {
	return []byte(`[
		{
			"id":"github","name":"GitHub","folderId":"folder-1","type":1,
			"revisionDate":"2024-01-30T00:00:00.000Z",
			"creationDate":"2023-12-01T00:00:00.000Z",
			"notes":"note-secret",
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
		},
		{
			"id":"empty","name":"Empty","type":1,
			"notes":"empty-note-secret",
			"login":{"username":"empty-user","password":""}
		},
		{
			"id":"trashed","name":"Trashed","type":1,
			"deletedDate":"2024-01-01T00:00:00.000Z",
			"login":{"password":"trash-secret"}
		}
	]`)
}

func mustJSON(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func assertNoSecrets(t *testing.T, got string) {
	t.Helper()
	for _, secret := range []string{
		"p@ss-should-not-leak",
		"danil.secret@example.com",
		"totp-should-not-leak",
		"note-secret",
		"history-secret",
		"uri-secret",
		"legacy-secret",
		"legacy-user",
		"empty-note-secret",
		"empty-user",
		"trash-secret",
		"aws-secret",
		"master-password",
		"client-secret",
		"sess-from-unlock",
		`"password"`,
		`"username"`,
		`"totp"`,
		`"pwned_count"`,
	} {
		if strings.Contains(got, secret) {
			t.Fatalf("output contains %q: %s", secret, got)
		}
	}
}

type scriptRunner struct {
	items    []byte
	folders  []byte
	status   string
	userID   string
	failLock bool
	calls    []string
	// holdSync, when set, blocks bw sync until the channel is closed.
	holdSync <-chan struct{}
}

func newScriptRunner(items []byte) *scriptRunner {
	return &scriptRunner{
		items:   items,
		folders: []byte(`[{"id":"folder-1","name":"Work"}]`),
		status:  bw.StatusLocked,
		userID:  "user-1",
	}
}

func (s *scriptRunner) Run(ctx context.Context, args []string, env []string) (bw.Result, error) {
	cmd := strings.Join(args, " ")
	s.calls = append(s.calls, cmd)
	if cmd == "sync" && s.holdSync != nil {
		select {
		case <-s.holdSync:
		case <-ctx.Done():
			return bw.Result{}, ctx.Err()
		}
	}
	if strings.Contains(cmd, "master-password") || strings.Contains(cmd, "sess-from-unlock") || strings.Contains(cmd, "client-secret") {
		return bw.Result{ExitCode: 1, Stderr: []byte("secret in argv")}, &bw.ExitError{ExitCode: 1}
	}
	switch cmd {
	case "status":
		if hasPrefix(env, "BW_SESSION=") || hasPrefix(env, "BW_PASSWORD=") {
			return bw.Result{ExitCode: 1, Stderr: []byte("status saw a secret")}, &bw.ExitError{ExitCode: 1}
		}
		body, err := json.Marshal(map[string]any{
			"status":    s.status,
			"userId":    s.userID,
			"serverUrl": "https://bw.example",
		})
		if err != nil {
			return bw.Result{}, err
		}
		return bw.Result{Stdout: body}, nil
	case "unlock --passwordenv BW_PASSWORD --raw":
		if !hasPrefix(env, "BW_PASSWORD=master-password") || hasPrefix(env, "BW_SESSION=") {
			return bw.Result{ExitCode: 1, Stderr: []byte("unlock env")}, &bw.ExitError{ExitCode: 1}
		}
		return bw.Result{Stdout: []byte("sess-from-unlock\n")}, nil
	case "sync", "list folders", "list items", "lock":
		if !hasPrefix(env, "BW_SESSION=sess-from-unlock") {
			return bw.Result{ExitCode: 1, Stderr: []byte("missing session")}, &bw.ExitError{ExitCode: 1}
		}
		if cmd == "lock" && s.failLock {
			return bw.Result{ExitCode: 1, Stderr: []byte("lock failed")}, &bw.ExitError{ExitCode: 1}
		}
		switch cmd {
		case "list folders":
			return bw.Result{Stdout: s.folders}, nil
		case "list items":
			return bw.Result{Stdout: s.items}, nil
		default:
			return bw.Result{}, nil
		}
	default:
		return bw.Result{ExitCode: 1, Stderr: []byte("unexpected " + cmd)}, &bw.ExitError{ExitCode: 1}
	}
}

func hasPrefix(env []string, prefix string) bool {
	for _, e := range env {
		if strings.HasPrefix(e, prefix) {
			return true
		}
	}
	return false
}
