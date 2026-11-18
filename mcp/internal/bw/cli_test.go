package bw

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestLoginAPIKeyPutsSecretsOnlyInChildEnv(t *testing.T) {
	runner := &fakeRunner{fn: func(args []string, env []string) (Result, error) {
		if strings.Join(args, " ") == "status" {
			return Result{Stdout: []byte(`{"status":"locked","userId":"user-1"}`)}, nil
		}
		return Result{}, nil
	}}
	cli := &CLI{Runner: runner, DataDir: "/var/lib/bw"}
	clientID := []byte("client-id")
	clientSecret := []byte("client-secret")

	st, err := cli.loginAPIKey(context.Background(), clientID, clientSecret)
	if err != nil {
		t.Fatal(err)
	}
	if st.Status != StatusLocked || st.UserID != "user-1" {
		t.Fatalf("status = %+v", st)
	}
	if len(runner.calls) != 2 {
		t.Fatalf("calls = %d, want 2", len(runner.calls))
	}
	call := runner.calls[0]
	if got, want := strings.Join(call.Args, " "), "login --apikey"; got != want {
		t.Fatalf("args = %q, want %q", got, want)
	}
	assertEnv(t, call.Env, []string{
		"BITWARDENCLI_APPDATA_DIR=/var/lib/bw",
		"BW_CLIENTID=client-id",
		"BW_CLIENTSECRET=client-secret",
	})
	assertArgsExclude(t, call.Args, "client-secret", "client-id", "BW_SESSION", "--session")
	if hasPrefix(call.Env, "BW_PASSWORD=") || hasPrefix(call.Env, "BW_SESSION=") {
		t.Fatalf("login env has password or session: %#v", call.Env)
	}
}

func TestUnlockPasswordIsEnvOnly(t *testing.T) {
	const password = "master-password"
	const session = "5PBYGU+5yt3RHcCjoeJKx/wByU34vokGRZjXpSH7Ylo8w=="
	runner := &fakeRunner{fn: func(args []string, env []string) (Result, error) {
		return Result{Stdout: []byte(session + "\n")}, nil
	}}
	cli := &CLI{Runner: runner, DataDir: "/var/lib/bw"}

	got, err := cli.unlock(context.Background(), []byte(password))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != session {
		t.Fatalf("session = %q", got)
	}
	call := runner.mustOne(t)
	if strings.Join(call.Args, " ") != "unlock --passwordenv BW_PASSWORD --raw" {
		t.Fatalf("args = %#v", call.Args)
	}
	assertEnv(t, call.Env, []string{
		"BITWARDENCLI_APPDATA_DIR=/var/lib/bw",
		"BW_PASSWORD=" + password,
	})
	assertArgsExclude(t, call.Args, password, session, "--session")
	if hasPrefix(call.Env, "BW_SESSION=") || hasPrefix(call.Env, "BW_CLIENTSECRET=") {
		t.Fatalf("unlock env = %#v", call.Env)
	}
}

func TestUnlockDoesNotLogUnexpectedOutput(t *testing.T) {
	const password = "master-password"
	runner := &fakeRunner{fn: func(args []string, env []string) (Result, error) {
		return Result{Stdout: []byte("export BW_SESSION=\"secret-key\"\n")}, nil
	}}
	cli := &CLI{Runner: runner, DataDir: "/var/lib/bw"}
	_, err := cli.unlock(context.Background(), []byte(password))
	if err == nil {
		t.Fatal("expected error")
	}
	if strings.Contains(err.Error(), password) || strings.Contains(err.Error(), "secret-key") || strings.Contains(err.Error(), "BW_SESSION") {
		t.Fatalf("error leaks unlock output: %v", err)
	}
}

func TestSessionCommandsUseBWSessionEnv(t *testing.T) {
	const session = "session-key"
	runner := &fakeRunner{fn: func(args []string, env []string) (Result, error) {
		switch strings.Join(args, " ") {
		case "list items":
			return Result{Stdout: []byte("[]")}, nil
		case "list folders":
			return Result{Stdout: []byte("[]")}, nil
		default:
			return Result{}, nil
		}
	}}
	cli := &CLI{Runner: runner, DataDir: "/var/lib/bw"}
	ctx := context.Background()
	if err := cli.sync(ctx, []byte(session)); err != nil {
		t.Fatal(err)
	}
	if _, err := cli.listRawItems(ctx, []byte(session)); err != nil {
		t.Fatal(err)
	}
	if _, err := cli.listFolders(ctx, []byte(session)); err != nil {
		t.Fatal(err)
	}
	if err := cli.lock(ctx, []byte(session)); err != nil {
		t.Fatal(err)
	}

	wantArgs := []string{"sync", "list items", "list folders", "lock"}
	if len(runner.calls) != len(wantArgs) {
		t.Fatalf("calls = %d, want %d", len(runner.calls), len(wantArgs))
	}
	for i, call := range runner.calls {
		if strings.Join(call.Args, " ") != wantArgs[i] {
			t.Fatalf("call %d args = %#v, want %q", i, call.Args, wantArgs[i])
		}
		assertEnv(t, call.Env, []string{
			"BITWARDENCLI_APPDATA_DIR=/var/lib/bw",
			"BW_SESSION=" + session,
		})
		assertArgsExclude(t, call.Args, session, "--session", "BW_PASSWORD", "BW_CLIENTSECRET")
	}
}

func TestStatusHasNoSession(t *testing.T) {
	runner := &fakeRunner{fn: func(args []string, env []string) (Result, error) {
		return Result{Stdout: []byte(`{
			"serverUrl":"https://bitwarden.example.com",
			"lastSync":null,
			"userEmail":null,
			"userId":null,
			"status":"unauthenticated"
		}`)}, nil
	}}
	cli := &CLI{Runner: runner, DataDir: "/var/lib/bw"}
	st, err := cli.Status(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if st.Status != StatusUnauthenticated || st.UserID != "" || st.ServerURL != "https://bitwarden.example.com" {
		t.Fatalf("status = %+v", st)
	}
	call := runner.mustOne(t)
	if strings.Join(call.Args, " ") != "status" {
		t.Fatalf("args = %#v", call.Args)
	}
	assertEnv(t, call.Env, []string{"BITWARDENCLI_APPDATA_DIR=/var/lib/bw"})
	if hasPrefix(call.Env, "BW_SESSION=") || hasPrefix(call.Env, "BW_PASSWORD=") || hasPrefix(call.Env, "BW_CLIENTSECRET=") {
		t.Fatalf("status env = %#v", call.Env)
	}
}

func TestListItemsFiltersLoginAndTrash(t *testing.T) {
	const password = "secret-1"
	const totp = "totp-secret"
	const trashed = "trash-secret"
	body := `[
		{
			"id":"login-1",
			"name":"GitHub",
			"folderId":"folder-1",
			"type":1,
			"deletedDate":null,
			"revisionDate":"2024-06-01T00:00:00.000Z",
			"creationDate":"2020-01-01T00:00:00.000Z",
			"notes":"do-not-keep",
			"login":{
				"username":"d@example.com",
				"password":"` + password + `",
				"totp":"` + totp + `",
				"passwordRevisionDate":"2024-01-02T03:04:05.000Z"
			}
		},
		{
			"id":"no-folder",
			"name":"Bare",
			"folderId":null,
			"type":1,
			"login":{"username":"a","password":"bare-secret","passwordRevisionDate":null}
		},
		{
			"id":"trashed",
			"name":"Old",
			"type":1,
			"deletedDate":"2024-01-01T00:00:00.000Z",
			"login":{"username":"a","password":"` + trashed + `"}
		},
		{"id":"note","name":"Note","type":2,"login":null}
	]`
	runner := &fakeRunner{fn: func(args []string, env []string) (Result, error) {
		return Result{Stdout: []byte(body)}, nil
	}}
	cli := &CLI{Runner: runner, DataDir: "/var/lib/bw"}
	items, err := cli.listRawItems(context.Background(), []byte("session-key"))
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 {
		t.Fatalf("items = %+v", items)
	}
	if items[0].ID != "login-1" || folderID(items[0].FolderID) != "folder-1" || items[0].Login == nil {
		t.Fatalf("first = %+v", items[0])
	}
	if string(items[0].Login.Password) != password || items[0].Login.PasswordRevisionDate == nil || *items[0].Login.PasswordRevisionDate != "2024-01-02T03:04:05.000Z" {
		t.Fatalf("first password metadata = %+v", items[0].Login)
	}
	if items[1].ID != "no-folder" || items[1].FolderID != nil || items[1].Login == nil || items[1].Login.PasswordRevisionDate != nil {
		t.Fatalf("second = %+v", items[1])
	}
	// The raw parse still holds the password. The model CLI returns does not encode it.
	if len(items[0].Login.Password) == 0 {
		t.Fatal("raw password was dropped before decode")
	}
}

func TestCommandErrorOmitsStdout(t *testing.T) {
	const password = "stdout-password"
	runner := &fakeRunner{fn: func(args []string, env []string) (Result, error) {
		return Result{Stdout: []byte(password), Stderr: []byte("locked"), ExitCode: 1}, &ExitError{ExitCode: 1}
	}}
	cli := &CLI{Runner: runner, DataDir: "/var/lib/bw"}
	_, err := cli.listRawItems(context.Background(), []byte("session-key"))
	if err == nil {
		t.Fatal("expected error")
	}
	if strings.Contains(err.Error(), password) {
		t.Fatalf("error contains stdout: %v", err)
	}
	if !strings.Contains(err.Error(), "locked") {
		t.Fatalf("error = %v, want stderr", err)
	}
}

func TestParseErrorOmitsStdout(t *testing.T) {
	const password = "not-json-password"
	runner := &fakeRunner{fn: func(args []string, env []string) (Result, error) {
		return Result{Stdout: []byte(password)}, nil
	}}
	cli := &CLI{Runner: runner, DataDir: "/var/lib/bw"}
	_, err := cli.listRawItems(context.Background(), []byte("session-key"))
	if err == nil {
		t.Fatal("expected error")
	}
	if strings.Contains(err.Error(), password) {
		t.Fatalf("error contains stdout: %v", err)
	}
}

func TestCLIRequiresDataDir(t *testing.T) {
	cli := &CLI{Runner: &fakeRunner{}}
	if _, err := cli.Status(context.Background()); err == nil {
		t.Fatal("expected error")
	}
}

func TestCLIRejectsNewlineInSecret(t *testing.T) {
	cli := &CLI{Runner: &fakeRunner{}, DataDir: "/var/lib/bw"}
	_, err := cli.loginAPIKey(context.Background(), []byte("id"), []byte("sec\nret"))
	if err == nil {
		t.Fatal("expected error")
	}
	if strings.Contains(err.Error(), "sec") {
		t.Fatalf("error contains secret: %v", err)
	}
}

type recordedCall struct {
	Args []string
	Env  []string
}

type fakeRunner struct {
	calls []recordedCall
	fn    func(args []string, env []string) (Result, error)
}

func (f *fakeRunner) Run(ctx context.Context, args []string, env []string) (Result, error) {
	f.calls = append(f.calls, recordedCall{
		Args: append([]string(nil), args...),
		Env:  append([]string(nil), env...),
	})
	if f.fn != nil {
		return f.fn(args, env)
	}
	return Result{}, nil
}

func (f *fakeRunner) mustOne(t *testing.T) recordedCall {
	t.Helper()
	if len(f.calls) != 1 {
		t.Fatalf("calls = %d, want 1", len(f.calls))
	}
	return f.calls[0]
}

func assertEnv(t *testing.T, got, want []string) {
	t.Helper()
	if strings.Join(got, "\x00") != strings.Join(want, "\x00") {
		t.Fatalf("env = %#v, want %#v", got, want)
	}
}

func assertArgsExclude(t *testing.T, args []string, forbidden ...string) {
	t.Helper()
	joined := strings.Join(args, " ")
	for _, f := range forbidden {
		if strings.Contains(joined, f) {
			t.Fatalf("args %q contain %q", joined, f)
		}
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

func TestListFolders(t *testing.T) {
	runner := &fakeRunner{fn: func(args []string, env []string) (Result, error) {
		return Result{Stdout: []byte(`[{"id":"folder-1","name":"Work"}]`)}, nil
	}}
	cli := &CLI{Runner: runner, DataDir: "/var/lib/bw"}
	folders, err := cli.listFolders(context.Background(), []byte("session-key"))
	if err != nil {
		t.Fatal(err)
	}
	if len(folders) != 1 || folders[0].ID != "folder-1" || folders[0].Name != "Work" {
		t.Fatalf("folders = %+v", folders)
	}
}

func TestRunnerErrorIsWrappedWithoutStdout(t *testing.T) {
	runner := &fakeRunner{fn: func(args []string, env []string) (Result, error) {
		return Result{}, errors.New("exec failed")
	}}
	cli := &CLI{Runner: runner, DataDir: "/var/lib/bw"}
	err := cli.sync(context.Background(), []byte("session-key"))
	if err == nil || !strings.Contains(err.Error(), "exec failed") {
		t.Fatalf("error = %v", err)
	}
}
func TestOpenLockedUnlocksAndCloseDropsToken(t *testing.T) {
	runner := &openRunner{
		statuses:   []string{StatusLocked},
		userIDs:    []string{"user-1"},
		unlockKeys: []string{"sess-1"},
	}
	cli := &CLI{Runner: runner, DataDir: "/var/lib/bw"}
	source := &testSecretSource{masterPassword: []byte("master-password")}
	if err := cli.Open(context.Background(), source); err != nil {
		t.Fatal(err)
	}
	assertSecretLoads(t, source.loaded, "master password")
	assertSecretBuffersWiped(t, source.buffers)
	if string(cli.session) != "sess-1" {
		t.Fatalf("open session %q", cli.session)
	}
	if err := cli.Sync(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := cli.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
	if cli.session != nil {
		t.Fatalf("token still present after close: %q", cli.session)
	}
	if err := cli.Sync(context.Background()); err == nil || !strings.Contains(err.Error(), "not open") {
		t.Fatalf("sync after close = %v", err)
	}
	assertCalls(t, runner.calls, []string{"status", "unlock --passwordenv BW_PASSWORD --raw", "sync", "lock"})
	if runner.lockedWith[0] != "sess-1" {
		t.Fatalf("locked with %q", runner.lockedWith)
	}
}

func TestOpenLoginThenUnlock(t *testing.T) {
	runner := &openRunner{
		statuses:   []string{StatusUnauthenticated, StatusLocked},
		userIDs:    []string{"", "user-after-login"},
		unlockKeys: []string{"sess-1"},
	}
	cli := &CLI{Runner: runner, DataDir: "/var/lib/bw"}
	source := testSecrets()
	if err := cli.Open(context.Background(), source); err != nil {
		t.Fatal(err)
	}
	assertSecretLoads(t, source.loaded, "client id", "client secret", "master password")
	assertSecretBuffersWiped(t, source.buffers)
	defer func() { _ = cli.Close(context.Background()) }()
	assertCalls(t, runner.calls, []string{"status", "login --apikey", "status", "unlock --passwordenv BW_PASSWORD --raw"})
	login := runner.envs[1]
	assertEnv(t, login, []string{
		"BITWARDENCLI_APPDATA_DIR=/var/lib/bw",
		"BW_CLIENTID=client-id",
		"BW_CLIENTSECRET=client-secret",
	})
	unlock := runner.envs[3]
	assertEnv(t, unlock, []string{
		"BITWARDENCLI_APPDATA_DIR=/var/lib/bw",
		"BW_PASSWORD=master-password",
	})
}

func TestOpenRejectsUnauthenticatedAfterLogin(t *testing.T) {
	runner := &openRunner{
		statuses: []string{StatusUnauthenticated, StatusUnauthenticated},
		userIDs:  []string{"", ""},
	}
	cli := &CLI{Runner: runner, DataDir: "/var/lib/bw"}
	source := testSecrets()
	err := cli.Open(context.Background(), source)
	if err == nil || !strings.Contains(err.Error(), "expected authenticated") {
		t.Fatalf("error = %v", err)
	}
	assertSecretLoads(t, source.loaded, "client id", "client secret")
	assertCalls(t, runner.calls, []string{"status", "login --apikey", "status"})
}

func TestOpenDoesNotRetryUnlock(t *testing.T) {
	runner := &openRunner{
		statuses:  []string{StatusLocked},
		userIDs:   []string{"user-1"},
		unlockErr: errors.New("bad password"),
	}
	cli := &CLI{Runner: runner, DataDir: "/var/lib/bw"}
	err := cli.Open(context.Background(), testSecrets())
	if err == nil || strings.Contains(err.Error(), "master-password") {
		t.Fatalf("error = %v", err)
	}
	if len(runner.lockedWith) != 0 || cli.session != nil {
		t.Fatalf("locks = %#v session = %q", runner.lockedWith, cli.session)
	}
	assertCalls(t, runner.calls, []string{"status", "unlock --passwordenv BW_PASSWORD --raw"})
}

func TestOpenUnlockedDoesNothing(t *testing.T) {
	runner := &openRunner{
		statuses: []string{StatusUnlocked},
		userIDs:  []string{"user-1"},
	}
	cli := &CLI{Runner: runner, DataDir: "/var/lib/bw"}
	source := &testSecretSource{}
	if err := cli.Open(context.Background(), source); err != nil {
		t.Fatal(err)
	}
	assertSecretLoads(t, source.loaded)
	if cli.session != nil {
		t.Fatalf("session = %q", cli.session)
	}
	assertCalls(t, runner.calls, []string{"status"})
}

func TestCloseLockFailureDropsToken(t *testing.T) {
	runner := &openRunner{
		statuses:   []string{StatusLocked},
		userIDs:    []string{"user-1"},
		unlockKeys: []string{"sess-1"},
		lockErr:    errors.New("lock failed"),
	}
	cli := &CLI{Runner: runner, DataDir: "/var/lib/bw"}
	if err := cli.Open(context.Background(), testSecrets()); err != nil {
		t.Fatal(err)
	}
	err := cli.Close(context.Background())
	if err == nil || !strings.Contains(err.Error(), "lock failed") {
		t.Fatalf("error = %v", err)
	}
	if cli.session != nil {
		t.Fatalf("token kept after failed close: %q", cli.session)
	}
	if strings.Contains(err.Error(), "sess-1") {
		t.Fatalf("error contains session: %v", err)
	}
	assertCalls(t, runner.calls, []string{"status", "unlock --passwordenv BW_PASSWORD --raw", "lock"})
}

func TestOpenRejectsEmptySecrets(t *testing.T) {
	runner := &openRunner{statuses: []string{StatusLocked}, userIDs: []string{"user-1"}}
	cli := &CLI{Runner: runner, DataDir: "/var/lib/bw"}
	source := &testSecretSource{}
	err := cli.Open(context.Background(), source)
	if err == nil || !strings.Contains(err.Error(), "master password is required") {
		t.Fatalf("error = %v", err)
	}
	assertSecretLoads(t, source.loaded, "master password")
	assertCalls(t, runner.calls, []string{"status"})
}

func TestOpenAlreadyOpenDoesNotUnlockAgain(t *testing.T) {
	runner := &openRunner{
		statuses:   []string{StatusLocked},
		userIDs:    []string{"user-1"},
		unlockKeys: []string{"sess-1", "sess-2"},
	}
	cli := &CLI{Runner: runner, DataDir: "/var/lib/bw"}
	if err := cli.Open(context.Background(), testSecrets()); err != nil {
		t.Fatal(err)
	}
	err := cli.Open(context.Background(), testSecrets())
	if err == nil || !strings.Contains(err.Error(), "already open") {
		t.Fatalf("error = %v", err)
	}
	if string(cli.session) != "sess-1" {
		t.Fatalf("session replaced: %q", cli.session)
	}
	if strings.Count(strings.Join(runner.calls, ","), "unlock") != 1 {
		t.Fatalf("calls = %#v", runner.calls)
	}
}

func TestOpenThenCloseThenOpenUsesNewToken(t *testing.T) {
	runner := &openRunner{
		statuses:   []string{StatusLocked, StatusLocked},
		userIDs:    []string{"user-1", "user-1"},
		unlockKeys: []string{"sess-1", "sess-2"},
	}
	cli := &CLI{Runner: runner, DataDir: "/var/lib/bw"}
	if err := cli.Open(context.Background(), testSecrets()); err != nil {
		t.Fatal(err)
	}
	if err := cli.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := cli.Open(context.Background(), testSecrets()); err != nil {
		t.Fatal(err)
	}
	if string(cli.session) != "sess-2" {
		t.Fatalf("session = %q, want a new token", cli.session)
	}
	if err := cli.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
	if runner.lockedWith[0] != "sess-1" || runner.lockedWith[1] != "sess-2" {
		t.Fatalf("locked with %#v", runner.lockedWith)
	}
}

func TestStatusWhileOpenDoesNotSendSession(t *testing.T) {
	runner := &openRunner{
		statuses:   []string{StatusLocked, StatusLocked},
		userIDs:    []string{"user-1", "user-1"},
		unlockKeys: []string{"sess-1"},
	}
	cli := &CLI{Runner: runner, DataDir: "/var/lib/bw"}
	if err := cli.Open(context.Background(), testSecrets()); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = cli.Close(context.Background()) }()
	st, err := cli.Status(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if st.Status != StatusLocked || st.UserID != "user-1" {
		t.Fatalf("status = %+v", st)
	}
	statusEnv := runner.envs[len(runner.envs)-1]
	if hasPrefix(statusEnv, "BW_SESSION=") || hasPrefix(statusEnv, "BW_PASSWORD=") {
		t.Fatalf("status env = %#v", statusEnv)
	}
}

func TestCloseIsNoopWhenNotOpen(t *testing.T) {
	runner := &openRunner{}
	cli := &CLI{Runner: runner, DataDir: "/var/lib/bw"}
	if err := cli.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(runner.calls) != 0 {
		t.Fatalf("calls = %#v", runner.calls)
	}
}

func TestOpenCanceled(t *testing.T) {
	runner := &openRunner{}
	cli := &CLI{Runner: runner, DataDir: "/var/lib/bw"}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := cli.Open(ctx, testSecrets())
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v", err)
	}
	if len(runner.calls) != 0 {
		t.Fatalf("calls = %#v", runner.calls)
	}
}

func testSecrets() *testSecretSource {
	return &testSecretSource{
		clientID:       []byte("client-id"),
		clientSecret:   []byte("client-secret"),
		masterPassword: []byte("master-password"),
	}
}

type testSecretSource struct {
	clientID       []byte
	clientSecret   []byte
	masterPassword []byte
	loaded         []string
	buffers        [][]byte
}

func (s *testSecretSource) LoadClientID() ([]byte, error) {
	return s.load("client id", s.clientID)
}

func (s *testSecretSource) LoadClientSecret() ([]byte, error) {
	return s.load("client secret", s.clientSecret)
}

func (s *testSecretSource) LoadMasterPassword() ([]byte, error) {
	return s.load("master password", s.masterPassword)
}

func (s *testSecretSource) load(name string, value []byte) ([]byte, error) {
	s.loaded = append(s.loaded, name)
	loaded := bytes.Clone(value)
	s.buffers = append(s.buffers, loaded)
	return loaded, nil
}

func assertSecretLoads(t *testing.T, got []string, want ...string) {
	t.Helper()
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("loaded secrets = %#v, want %#v", got, want)
	}
}

func assertSecretBuffersWiped(t *testing.T, buffers [][]byte) {
	t.Helper()
	for i, buffer := range buffers {
		for _, b := range buffer {
			if b != 0 {
				t.Fatalf("secret buffer %d was not wiped", i)
			}
		}
	}
}

func assertCalls(t *testing.T, got, want []string) {
	t.Helper()
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("calls = %#v, want %#v", got, want)
	}
}

type openRunner struct {
	statuses   []string
	userIDs    []string
	statusN    int
	unlockKeys []string
	unlockN    int
	unlockErr  error
	lockErr    error
	calls      []string
	envs       [][]string
	lockedWith []string
}

func (o *openRunner) Run(ctx context.Context, args []string, env []string) (Result, error) {
	cmd := strings.Join(args, " ")
	o.calls = append(o.calls, cmd)
	o.envs = append(o.envs, append([]string(nil), env...))
	switch cmd {
	case "status":
		if hasPrefix(env, "BW_SESSION=") || hasPrefix(env, "BW_PASSWORD=") || hasPrefix(env, "BW_CLIENTSECRET=") {
			return Result{ExitCode: 1, Stderr: []byte("status saw a secret")}, &ExitError{ExitCode: 1}
		}
		if o.statusN >= len(o.statuses) {
			return Result{ExitCode: 1, Stderr: []byte("unexpected status")}, &ExitError{ExitCode: 1}
		}
		st := o.statuses[o.statusN]
		user := ""
		if o.statusN < len(o.userIDs) {
			user = o.userIDs[o.statusN]
		}
		o.statusN++
		body := `{"status":"` + st + `","serverUrl":"https://bw.example"`
		if user == "" {
			body += `,"userId":null}`
		} else {
			body += `,"userId":"` + user + `"}`
		}
		return Result{Stdout: []byte(body)}, nil
	case "login --apikey":
		if !hasPrefix(env, "BW_CLIENTID=client-id") || !hasPrefix(env, "BW_CLIENTSECRET=client-secret") {
			return Result{ExitCode: 1, Stderr: []byte("login env")}, &ExitError{ExitCode: 1}
		}
		if hasPrefix(env, "BW_PASSWORD=") || hasPrefix(env, "BW_SESSION=") {
			return Result{ExitCode: 1, Stderr: []byte("login saw password or session")}, &ExitError{ExitCode: 1}
		}
		return Result{}, nil
	case "unlock --passwordenv BW_PASSWORD --raw":
		if o.unlockErr != nil {
			return Result{ExitCode: 1, Stderr: []byte(o.unlockErr.Error())}, &ExitError{ExitCode: 1}
		}
		if !hasPrefix(env, "BW_PASSWORD=master-password") || hasPrefix(env, "BW_SESSION=") {
			return Result{ExitCode: 1, Stderr: []byte("unlock env")}, &ExitError{ExitCode: 1}
		}
		if o.unlockN >= len(o.unlockKeys) {
			return Result{ExitCode: 1, Stderr: []byte("unexpected unlock")}, &ExitError{ExitCode: 1}
		}
		key := o.unlockKeys[o.unlockN]
		o.unlockN++
		return Result{Stdout: []byte(key + "\n")}, nil
	case "sync":
		if !hasPrefix(env, "BW_SESSION=") {
			return Result{ExitCode: 1, Stderr: []byte("missing session")}, &ExitError{ExitCode: 1}
		}
		return Result{}, nil
	case "lock":
		for _, e := range env {
			if strings.HasPrefix(e, "BW_SESSION=") {
				o.lockedWith = append(o.lockedWith, strings.TrimPrefix(e, "BW_SESSION="))
			}
		}
		if o.lockErr != nil {
			return Result{ExitCode: 1, Stderr: []byte(o.lockErr.Error())}, &ExitError{ExitCode: 1}
		}
		return Result{}, nil
	default:
		return Result{ExitCode: 1, Stderr: []byte("unexpected " + cmd)}, &ExitError{ExitCode: 1}
	}
}
func TestReadProjectsItems(t *testing.T) {
	const (
		password     = `cur"rent\secret`
		username     = "d@example.com"
		legacyPass   = "legacy-secret"
		unknownPass  = "unknown-secret"
		futurePass   = "future-secret"
		historyPass  = "history-secret"
		emptyHistory = "empty-history-secret"
		totp         = "totp-secret"
		notes        = "note-secret"
		uri          = "https://secret.example/uri-secret"
		field        = "field-secret"
		attachment   = "attach-secret"
		trashPass    = "trash-secret"
		noteType     = "note-type-secret"
		noLogin      = "no-login-secret"
		folderID     = "folder-1"
		missingID    = "missing-folder-id"
		sessionKey   = "session-key-do-not-keep"
	)
	now := mustTime(t, "2024-01-31T00:00:00.000Z")

	items := []any{
		map[string]any{
			"id":           "github",
			"name":         "GitHub",
			"folderId":     folderID,
			"type":         1,
			"deletedDate":  nil,
			"revisionDate": "2024-01-30T00:00:00.000Z",
			"creationDate": "2023-12-01T00:00:00.000Z",
			"notes":        notes,
			"fields":       []any{map[string]any{"name": "pin", "value": field, "type": 0}},
			"attachments":  []any{map[string]any{"fileName": attachment, "url": "https://example.invalid/" + attachment}},
			"passwordHistory": []any{map[string]any{
				"password":     historyPass,
				"lastUsedDate": "2023-01-01T00:00:00.000Z",
			}},
			"login": map[string]any{
				"username":             username,
				"password":             password,
				"totp":                 totp,
				"passwordRevisionDate": "2024-01-01T00:00:00.000Z",
				"uris":                 []any{map[string]any{"uri": uri}},
			},
		},
		map[string]any{
			"id":           "legacy",
			"name":         "Legacy",
			"folderId":     nil,
			"type":         1,
			"revisionDate": "2024-01-30T00:00:00.000Z",
			"creationDate": "2023-12-01T00:00:00.000Z",
			"login": map[string]any{
				"username":             "legacy-user",
				"password":             legacyPass,
				"passwordRevisionDate": nil,
			},
		},
		map[string]any{
			"id":       "empty",
			"name":     "Empty",
			"type":     1,
			"folderId": folderID,
			"notes":    emptyHistory,
			"passwordHistory": []any{map[string]any{
				"password": emptyHistory,
			}},
			"login": map[string]any{
				"username": "empty-user",
				"password": "",
			},
		},
		map[string]any{
			"id":    "null-password",
			"name":  "NullPassword",
			"type":  1,
			"login": map[string]any{"username": "n", "password": nil},
		},
		map[string]any{
			"id":    "no-login",
			"name":  "NoLogin",
			"type":  1,
			"notes": noLogin,
		},
		map[string]any{
			"id":          "trashed",
			"name":        "Trashed",
			"type":        1,
			"deletedDate": "2024-01-01T00:00:00.000Z",
			"login":       map[string]any{"password": trashPass},
		},
		map[string]any{
			"id":    "note",
			"name":  "Note",
			"type":  2,
			"notes": noteType,
		},
		map[string]any{
			"id":           "unknown",
			"name":         "Unknown",
			"folderId":     missingID,
			"type":         1,
			"creationDate": "2020-01-01T00:00:00.000Z",
			"revisionDate": "2024-01-30T00:00:00.000Z",
			"login": map[string]any{
				"password":             unknownPass,
				"passwordRevisionDate": "2024-01-21T00:00:00.000Z",
			},
		},
		map[string]any{
			"id":           "future",
			"name":         "Future",
			"type":         1,
			"creationDate": "2020-01-01T00:00:00.000Z",
			"revisionDate": "2024-01-30T00:00:00.000Z",
			"login": map[string]any{
				"password":             futurePass,
				"passwordRevisionDate": "2024-02-01T00:00:00.000Z",
			},
		},
	}
	folders := []any{
		map[string]any{"id": folderID, "name": "Work"},
		map[string]any{"id": "folder-unused", "name": "Personal"},
		map[string]any{"id": "", "name": "ignored"},
	}

	got := readCLI(t, items, folders, []byte(sessionKey), now)
	if len(got) != 4 {
		t.Fatalf("items = %d, want 4", len(got))
	}

	gotIDs := []string{got[0].ID, got[1].ID, got[2].ID, got[3].ID}
	wantIDs := []string{"github", "legacy", "unknown", "future"}
	if strings.Join(gotIDs, ",") != strings.Join(wantIDs, ",") {
		t.Fatalf("ids = %#v", gotIDs)
	}

	github := got[0]
	if github.Name != "GitHub" {
		t.Fatalf("github = %+v", github)
	}
	if github.Folder == nil || *github.Folder != "Work" {
		t.Fatalf("folder = %v", github.Folder)
	}
	if github.AgeDays != 30 {
		t.Fatalf("age = %d, want 30 from passwordRevisionDate", github.AgeDays)
	}

	legacy := got[1]
	if legacy.Folder != nil {
		t.Fatalf("legacy folder = %v", *legacy.Folder)
	}
	if legacy.AgeDays != 61 {
		t.Fatalf("legacy age = %d, want 61 from creationDate", legacy.AgeDays)
	}
	if got[2].Folder != nil {
		t.Fatalf("unknown folder = %v, want null", *got[2].Folder)
	}
	if got[2].AgeDays != 10 {
		t.Fatalf("unknown age = %d, want 10", got[2].AgeDays)
	}
	if got[3].AgeDays != 0 {
		t.Fatalf("future age = %d, want 0", got[3].AgeDays)
	}

	secrets := []string{
		password, username, legacyPass, unknownPass, futurePass,
		historyPass, emptyHistory, totp, notes, uri, field, attachment,
		trashPass, noteType, noLogin, missingID, sessionKey, "legacy-user",
		"revisionDate", "passwordRevisionDate", "totp",
	}
	publicJSON := mustJSON(t, got)
	assertNoSecrets(t, string(publicJSON), secrets)
	assertNoSecrets(t, fmt.Sprintf("%v %+v %#v", got, got, got), secrets)
	var decoded []map[string]any
	if err := json.Unmarshal(publicJSON, &decoded); err != nil {
		t.Fatal(err)
	}
	for _, item := range decoded {
		for k := range item {
			switch k {
			case "id", "name", "masked_username", "folder", "age_days", "length", "entropy", "strength", "hash":
			default:
				t.Fatalf("unexpected public field %s", k)
			}
		}
	}

}

func TestReadAgeBoundaries(t *testing.T) {
	now := mustTime(t, "2024-01-31T00:00:00Z")
	cases := []struct {
		name string
		when string
		want int
	}{
		{name: "exact day", when: "2024-01-30T00:00:00Z", want: 1},
		{name: "just under a day", when: "2024-01-30T00:00:00.000000001Z", want: 0},
		{name: "millis", when: "2024-01-01T00:00:00.000Z", want: 30},
		{name: "equal", when: "2024-01-31T00:00:00Z", want: 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			items := []any{map[string]any{
				"id":           "a",
				"name":         "A",
				"type":         1,
				"creationDate": "2019-01-01T00:00:00Z",
				"revisionDate": "2024-01-30T00:00:00Z",
				"login": map[string]any{
					"password":             "boundary-secret",
					"passwordRevisionDate": tc.when,
				},
			}}
			got := readCLI(t, items, []any{}, []byte("session-key"), now)
			if len(got) != 1 || got[0].AgeDays != tc.want {
				t.Fatalf("age = %v, want %d", ages(got), tc.want)
			}
		})
	}
}

func TestReadMissingDateDoesNotUseRevisionDate(t *testing.T) {
	items := []any{map[string]any{
		"id":           "nodate",
		"name":         "NoDate",
		"type":         1,
		"revisionDate": "2024-01-30T00:00:00.000Z",
		"login":        map[string]any{"password": "nodate-secret"},
	}}
	_, err := readCLIErr(t, items, []any{}, []byte("session-key"), mustTime(t, "2024-01-31T00:00:00Z"))
	if err == nil {
		t.Fatal("expected error")
	}
	if strings.Contains(err.Error(), "nodate-secret") {
		t.Fatalf("error contains password: %v", err)
	}
	if !strings.Contains(err.Error(), "nodate") || !strings.Contains(err.Error(), "missing passwordRevisionDate") {
		t.Fatalf("error = %v", err)
	}
}

func TestReadInvalidDate(t *testing.T) {
	items := []any{map[string]any{
		"id":           "bad",
		"name":         "Bad",
		"type":         1,
		"creationDate": "2020-01-01T00:00:00.000Z",
		"revisionDate": "2024-01-30T00:00:00.000Z",
		"login": map[string]any{
			"password":             "bad-secret",
			"passwordRevisionDate": "yesterday",
		},
	}}
	_, err := readCLIErr(t, items, []any{}, []byte("session-key"), mustTime(t, "2024-01-31T00:00:00Z"))
	if err == nil {
		t.Fatal("expected error")
	}
	if strings.Contains(err.Error(), "bad-secret") {
		t.Fatalf("error contains password: %v", err)
	}
	if !strings.Contains(err.Error(), "bad") || !strings.Contains(err.Error(), "passwordRevisionDate") {
		t.Fatalf("error = %v", err)
	}
}

func TestReadStopsOnDateError(t *testing.T) {
	items := []any{
		map[string]any{
			"id":           "ok",
			"name":         "Ok",
			"type":         1,
			"creationDate": "2024-01-01T00:00:00Z",
			"login":        map[string]any{"password": "ok-secret"},
		},
		map[string]any{
			"id":           "bad",
			"name":         "Bad",
			"type":         1,
			"revisionDate": "2024-01-30T00:00:00Z",
			"login":        map[string]any{"password": "bad-secret"},
		},
	}
	_, err := readCLIErr(t, items, []any{}, []byte("session-key"), mustTime(t, "2024-01-31T00:00:00Z"))
	if err == nil || !strings.Contains(err.Error(), "bad") {
		t.Fatalf("error = %v", err)
	}
	if strings.Contains(err.Error(), "ok-secret") || strings.Contains(err.Error(), "bad-secret") {
		t.Fatalf("error contains password: %v", err)
	}
}

func TestOpenSyncReadClose(t *testing.T) {
	now := mustTime(t, "2024-01-31T00:00:00Z")
	items, err := json.Marshal([]any{map[string]any{
		"id":           "github",
		"name":         "GitHub",
		"folderId":     "folder-1",
		"type":         1,
		"creationDate": "2020-01-01T00:00:00Z",
		"revisionDate": "2024-01-30T00:00:00Z",
		"login": map[string]any{
			"username":             "d@example.com",
			"password":             "inside-secret",
			"passwordRevisionDate": "2024-01-01T00:00:00Z",
		},
	}})
	if err != nil {
		t.Fatal(err)
	}
	folders, err := json.Marshal([]any{map[string]any{"id": "folder-1", "name": "Work"}})
	if err != nil {
		t.Fatal(err)
	}
	runner := &scriptRunner{items: items, folders: folders}
	cli := &CLI{Runner: runner, DataDir: "/var/lib/bw"}
	if err := cli.Open(context.Background(), testSecrets()); err != nil {
		t.Fatal(err)
	}
	if err := cli.Sync(context.Background()); err != nil {
		t.Fatal(err)
	}
	got, err := cli.Read(context.Background(), now)
	if err != nil {
		t.Fatal(err)
	}
	if err := cli.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
	if cli.session != nil {
		t.Fatal("session token kept after close")
	}
	if len(got) != 1 || got[0].AgeDays != 30 || got[0].Folder == nil || *got[0].Folder != "Work" {
		t.Fatalf("items = %#v", got)
	}
	if strings.Contains(string(mustJSON(t, got)), "inside-secret") {
		t.Fatal("password left the CLI read layer")
	}
	wantCalls := []string{"status", "unlock --passwordenv BW_PASSWORD --raw", "sync", "list folders", "list items", "lock"}
	if strings.Join(runner.calls, ",") != strings.Join(wantCalls, ",") {
		t.Fatalf("calls = %#v", runner.calls)
	}
}

func TestReadListsFoldersBeforeItems(t *testing.T) {
	runner := &orderRunner{}
	cli := &CLI{Runner: runner, DataDir: "/var/lib/bw", session: []byte("sess-value")}
	items, err := cli.Read(context.Background(), mustTime(t, "2024-01-31T00:00:00Z"))
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 0 {
		t.Fatalf("items = %+v", items)
	}
	if strings.Join(runner.calls, ",") != "list folders,list items" {
		t.Fatalf("calls = %#v", runner.calls)
	}
	if !bytes.Equal(runner.sessions[0], []byte("sess-value")) || !bytes.Equal(runner.sessions[1], []byte("sess-value")) {
		t.Fatalf("sessions = %q %q", runner.sessions[0], runner.sessions[1])
	}
}

func TestReadWrapsListErrors(t *testing.T) {
	now := mustTime(t, "2024-01-31T00:00:00Z")
	folders := &failRunner{errOn: "list folders", msg: "folders down"}
	cli := &CLI{Runner: folders, DataDir: "/var/lib/bw", session: []byte("sess-value")}
	_, err := cli.Read(context.Background(), now)
	if err == nil || !strings.Contains(err.Error(), "list folders") || !strings.Contains(err.Error(), "folders down") {
		t.Fatalf("error = %v", err)
	}
	if strings.Join(folders.calls, ",") != "list folders" {
		t.Fatalf("calls = %#v", folders.calls)
	}

	items := &failRunner{errOn: "list items", msg: "items down"}
	cli = &CLI{Runner: items, DataDir: "/var/lib/bw", session: []byte("sess-value")}
	_, err = cli.Read(context.Background(), now)
	if err == nil || !strings.Contains(err.Error(), "list items") || !strings.Contains(err.Error(), "items down") {
		t.Fatalf("error = %v", err)
	}
	if strings.Join(items.calls, ",") != "list folders,list items" {
		t.Fatalf("calls = %#v", items.calls)
	}
}

func TestReadRejectsBadArgs(t *testing.T) {
	now := mustTime(t, "2024-01-31T00:00:00Z")
	r := &orderRunner{}
	t.Run("not open", func(t *testing.T) {
		cli := &CLI{Runner: r, DataDir: "/var/lib/bw"}
		_, err := cli.Read(context.Background(), now)
		if err == nil || !strings.Contains(err.Error(), "not open") {
			t.Fatalf("error = %v", err)
		}
		if len(r.calls) != 0 {
			t.Fatalf("calls = %#v", r.calls)
		}
	})
	t.Run("zero now", func(t *testing.T) {
		cli := &CLI{Runner: r, DataDir: "/var/lib/bw", session: []byte("s")}
		_, err := cli.Read(context.Background(), time.Time{})
		if err == nil || !strings.Contains(err.Error(), "now is zero") {
			t.Fatalf("error = %v", err)
		}
		if len(r.calls) != 0 {
			t.Fatalf("calls = %#v", r.calls)
		}
	})
}

func TestReadCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	r := &orderRunner{}
	cli := &CLI{Runner: r, DataDir: "/var/lib/bw", session: []byte("sess-value")}
	_, err := cli.Read(ctx, mustTime(t, "2024-01-31T00:00:00Z"))
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v", err)
	}
	if len(r.calls) != 0 {
		t.Fatalf("calls = %#v", r.calls)
	}
}

func readCLI(t *testing.T, items, folders []any, session []byte, now time.Time) []Login {
	t.Helper()
	v, err := readCLIErr(t, items, folders, session, now)
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func readCLIErr(t *testing.T, items, folders []any, session []byte, now time.Time) ([]Login, error) {
	t.Helper()
	itemBody, err := json.Marshal(items)
	if err != nil {
		t.Fatal(err)
	}
	folderBody, err := json.Marshal(folders)
	if err != nil {
		t.Fatal(err)
	}
	cli := &CLI{
		Runner: &mapRunner{out: map[string][]byte{
			"list items":   itemBody,
			"list folders": folderBody,
		}},
		DataDir: "/var/lib/bw",
		session: append([]byte(nil), session...),
	}
	return cli.Read(context.Background(), now)
}

func mustTime(t *testing.T, raw string) time.Time {
	t.Helper()
	parsed, err := time.Parse(time.RFC3339Nano, raw)
	if err != nil {
		t.Fatal(err)
	}
	return parsed
}

func mustJSON(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func assertNoSecrets(t *testing.T, got string, secrets []string) {
	t.Helper()
	for _, secret := range secrets {
		if secret != "" && strings.Contains(got, secret) {
			t.Fatalf("output contains %q: %s", secret, got)
		}
	}
}

func ages(items []Login) []int {
	out := make([]int, len(items))
	for i := range items {
		out[i] = items[i].AgeDays
	}
	return out
}

type orderRunner struct {
	calls    []string
	sessions [][]byte
}

func (o *orderRunner) Run(ctx context.Context, args []string, env []string) (Result, error) {
	o.calls = append(o.calls, strings.Join(args, " "))
	for _, e := range env {
		if strings.HasPrefix(e, "BW_SESSION=") {
			o.sessions = append(o.sessions, []byte(strings.TrimPrefix(e, "BW_SESSION=")))
		}
	}
	return Result{Stdout: []byte("[]")}, nil
}

type failRunner struct {
	errOn string
	msg   string
	calls []string
}

func (f *failRunner) Run(ctx context.Context, args []string, env []string) (Result, error) {
	cmd := strings.Join(args, " ")
	f.calls = append(f.calls, cmd)
	if cmd == f.errOn {
		return Result{}, errors.New(f.msg)
	}
	return Result{Stdout: []byte("[]")}, nil
}

type scriptRunner struct {
	items   []byte
	folders []byte
	calls   []string
}

func (s *scriptRunner) Run(ctx context.Context, args []string, env []string) (Result, error) {
	cmd := strings.Join(args, " ")
	s.calls = append(s.calls, cmd)
	switch cmd {
	case "status":
		if hasPrefix(env, "BW_SESSION=") {
			return fail("status saw session")
		}
		return Result{Stdout: []byte(`{"status":"locked","userId":"user-1","serverUrl":"https://bw.example"}`)}, nil
	case "unlock --passwordenv BW_PASSWORD --raw":
		if !hasPrefix(env, "BW_PASSWORD=master-password") || hasPrefix(env, "BW_SESSION=") {
			return fail("unlock env")
		}
		return Result{Stdout: []byte("sess-from-unlock\n")}, nil
	case "sync", "lock", "list folders", "list items":
		if !hasPrefix(env, "BW_SESSION=sess-from-unlock") {
			return fail("missing session")
		}
		switch cmd {
		case "list folders":
			return Result{Stdout: s.folders}, nil
		case "list items":
			return Result{Stdout: s.items}, nil
		default:
			return Result{}, nil
		}
	default:
		return fail("unexpected " + cmd)
	}
}

func fail(msg string) (Result, error) {
	return Result{ExitCode: 1, Stderr: []byte(msg)}, &ExitError{ExitCode: 1}
}

type mapRunner struct {
	out map[string][]byte
}

func (m *mapRunner) Run(ctx context.Context, args []string, env []string) (Result, error) {
	body, ok := m.out[strings.Join(args, " ")]
	if !ok {
		return Result{ExitCode: 1, Stderr: []byte("unexpected")}, &ExitError{ExitCode: 1}
	}
	return Result{Stdout: body}, nil
}
