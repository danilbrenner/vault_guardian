package bw

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"bw-mcp/internal/secrets"
)

// CLI is a Bitwarden CLI wrapper.
//
// Open authenticates if needed and unlocks a locked vault, keeping the session token inside CLI.
// Close locks and removes that token. The token is not returned and must not be logged.
// Sync and Read use the open token and return the vault model; callers do not pass a session key.
//
// Secrets are passed only through the child environment of the matching command.
type CLI struct {
	Runner Runner
	// DataDir is BITWARDENCLI_APPDATA_DIR. Required.
	// It must be private to the MCP container so a locked login survives the process
	// and the agent cannot read it.
	DataDir string

	mu      sync.Mutex
	session []byte
}

// Open logs in if needed, then unlocks a locked vault. It is a no-op when the
// vault is already unlocked. The session token stays on CLI after unlocking.
// It does not sync or retry. A new unlock invalidates older tokens, so Open
// fails if the CLI is already open. Credentials are loaded only for the
// operations needed and their byte buffers are wiped after use. Close is safe
// to call if Open fails.
func (c *CLI) Open(ctx context.Context, source secrets.Source) error {
	if c == nil {
		return errors.New("bw: cli is nil")
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	if len(c.session) != 0 {
		return errors.New("bw: already open")
	}
	st, err := c.Status(ctx)
	if err != nil {
		return err
	}
	if st.Status == StatusUnauthenticated {
		st, err = c.loginAPIKeyFromSource(ctx, source)
	}
	if err != nil {
		return err
	}
	if st.Status == StatusUnlocked {
		return nil
	}
	if st.Status != StatusLocked {
		return fmt.Errorf("bw status: expected authenticated, got %q", st.Status)
	}
	key, err := c.unlockFromSource(ctx, source)
	if err != nil {
		return err
	}
	c.session = key
	return nil
}

func (c *CLI) loginAPIKeyFromSource(ctx context.Context, source secrets.Source) (Status, error) {
	if source == nil {
		return Status{}, errors.New("bw: secret source is nil")
	}
	clientID, err := source.LoadClientID()
	if err != nil {
		return Status{}, err
	}
	defer wipe(clientID)
	if len(clientID) == 0 {
		return Status{}, errors.New("secrets: client id is empty")
	}
	clientSecret, err := source.LoadClientSecret()
	if err != nil {
		return Status{}, err
	}
	defer wipe(clientSecret)
	if err := c.loginAPIKeyCommand(ctx, clientID, clientSecret); err != nil {
		return Status{}, err
	}
	wipe(clientID)
	clientID = nil
	wipe(clientSecret)
	clientSecret = nil
	return c.Status(ctx)
}

func (c *CLI) unlockFromSource(ctx context.Context, source secrets.Source) ([]byte, error) {
	if source == nil {
		return nil, errors.New("bw: secret source is nil")
	}
	masterPassword, err := source.LoadMasterPassword()
	if err != nil {
		return nil, err
	}
	defer wipe(masterPassword)
	return c.unlock(ctx, masterPassword)
}

// Close locks the CLI and removes the session token.
// It is a no-op when the CLI is not open, including after a failed Open.
// The token is wiped whether locking succeeds or fails.
func (c *CLI) Close(ctx context.Context) error {
	if c == nil {
		return errors.New("bw: cli is nil")
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.session) == 0 {
		return nil
	}
	token := c.session
	c.session = nil
	defer wipe(token)
	return c.lock(ctx, token)
}

// Sync runs bw sync with the open session token.
func (c *CLI) Sync(ctx context.Context) error {
	if c == nil {
		return errors.New("bw: cli is nil")
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.session) == 0 {
		return errors.New("bw: not open")
	}
	return c.sync(ctx, c.session)
}

// Read lists folders and items and returns only their public projection.
// The CLI must be open. Read does not sync and does not take a session key.
//
// Items with no password are skipped. Age is whole days since
// login.passwordRevisionDate, or creationDate when that is absent.
// Item revisionDate is not an age source.
// TOTP, notes, URIs, custom fields, attachments, and password history are not kept.
// Passwords are used only to filter items; they are never included in the result.
func (c *CLI) Read(ctx context.Context, now time.Time) ([]Item, error) {
	if c == nil {
		return nil, errors.New("bw: cli is nil")
	}
	if len(c.session) == 0 {
		return nil, errors.New("bw: not open")
	}
	if now.IsZero() {
		return nil, errors.New("bw: now is zero")
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	items, err := c.readItems(ctx, c.session, now)
	if err != nil {
		return nil, err
	}
	return items, nil
}

// Status runs bw status with no session in the child environment.
// It does not require Open, and it does not send the open session token.
func (c *CLI) Status(ctx context.Context) (Status, error) {
	env, err := c.baseEnv()
	if err != nil {
		return Status{}, err
	}
	return getJSON[Status](c, ctx, []string{"status"}, env)
}

// loginAPIKey runs bw login --apikey, then returns the updated status.
// clientID and clientSecret are sent only as BW_CLIENTID and BW_CLIENTSECRET on that child.
func (c *CLI) loginAPIKey(ctx context.Context, clientID, clientSecret []byte) (Status, error) {
	if err := c.loginAPIKeyCommand(ctx, clientID, clientSecret); err != nil {
		return Status{}, err
	}
	return c.Status(ctx)
}

func (c *CLI) loginAPIKeyCommand(ctx context.Context, clientID, clientSecret []byte) error {
	if len(clientID) == 0 || len(clientSecret) == 0 {
		return errors.New("bw login: client id and client secret are required")
	}
	env, err := c.baseEnv()
	if err != nil {
		return err
	}
	id, err := envPair("BW_CLIENTID", clientID)
	if err != nil {
		return err
	}
	secret, err := envPair("BW_CLIENTSECRET", clientSecret)
	if err != nil {
		id = ""
		return err
	}
	_, runErr := c.run(ctx, []string{"login", "--apikey"}, append(env, id, secret))
	id = ""
	secret = ""
	if runErr != nil {
		return runErr
	}
	return nil
}

// unlock runs bw unlock --passwordenv BW_PASSWORD --raw.
// The master password is sent only as BW_PASSWORD on that child.
// The returned session key is stdout. It is not logged.
func (c *CLI) unlock(ctx context.Context, masterPassword []byte) ([]byte, error) {
	if len(masterPassword) == 0 {
		return nil, errors.New("bw unlock: master password is required")
	}
	env, err := c.baseEnv()
	if err != nil {
		return nil, err
	}
	pair, err := envPair("BW_PASSWORD", masterPassword)
	if err != nil {
		return nil, err
	}
	res, runErr := c.run(ctx, []string{"unlock", "--passwordenv", "BW_PASSWORD", "--raw"}, append(env, pair))
	pair = ""
	if runErr != nil {
		return nil, runErr
	}
	key := bytes.TrimSpace(res.Stdout)
	if len(key) == 0 || bytes.ContainsAny(key, "\r\n \"") || bytes.Contains(key, []byte("BW_SESSION")) {
		return nil, errors.New("bw unlock: unexpected session output")
	}
	return bytes.Clone(key), nil
}

func (c *CLI) sync(ctx context.Context, session []byte) error {
	env, err := c.sessionEnv(session)
	if err != nil {
		return err
	}
	_, err = c.run(ctx, []string{"sync"}, env)
	return err
}

func (c *CLI) listRawItems(ctx context.Context, session []byte) ([]rawItem, error) {
	env, err := c.sessionEnv(session)
	if err != nil {
		return nil, err
	}
	raw, err := getJSON[[]rawItem](c, ctx, []string{"list", "items"}, env)
	if err != nil {
		return nil, err
	}
	items := make([]rawItem, 0, len(raw))
	for _, it := range raw {
		if it.Type != itemTypeLogin || it.DeletedDate != nil {
			continue
		}
		items = append(items, it)
	}
	return items, nil
}

func (c *CLI) listFolders(ctx context.Context, session []byte) ([]folder, error) {
	env, err := c.sessionEnv(session)
	if err != nil {
		return nil, err
	}
	folders, err := getJSON[[]folder](c, ctx, []string{"list", "folders"}, env)
	if err != nil {
		return nil, err
	}
	return folders, nil
}

func (c *CLI) lock(ctx context.Context, session []byte) error {
	env, err := c.sessionEnv(session)
	if err != nil {
		return err
	}
	_, err = c.run(ctx, []string{"lock"}, env)
	return err
}

func (c *CLI) baseEnv() ([]string, error) {
	if c.DataDir == "" {
		return nil, errors.New("bw: data dir is required")
	}
	if strings.ContainsAny(c.DataDir, "\x00\n\r") {
		return nil, errors.New("bw: data dir contains a newline or NUL")
	}
	return []string{"BITWARDENCLI_APPDATA_DIR=" + c.DataDir}, nil
}

func (c *CLI) sessionEnv(session []byte) ([]string, error) {
	if len(session) == 0 {
		return nil, errors.New("bw: session key is empty")
	}
	env, err := c.baseEnv()
	if err != nil {
		return nil, err
	}
	pair, err := envPair("BW_SESSION", session)
	if err != nil {
		return nil, err
	}
	return append(env, pair), nil
}

func envPair(key string, val []byte) (string, error) {
	if bytes.ContainsAny(val, "\x00\n\r") {
		return "", fmt.Errorf("bw: %s contains a newline or NUL", key)
	}
	return key + "=" + string(val), nil
}

func getJSON[T any](c *CLI, ctx context.Context, args []string, env []string) (T, error) {
	var dest T
	res, err := c.run(ctx, args, env)
	if err != nil {
		return dest, err
	}
	if err := json.Unmarshal(res.Stdout, &dest); err != nil {
		return dest, fmt.Errorf("bw %s: parse json: %w", strings.Join(args, " "), err)
	}
	return dest, nil
}

func (c *CLI) run(ctx context.Context, args []string, env []string) (Result, error) {
	defer clearEnv(env)
	if c.Runner == nil {
		return Result{}, errors.New("bw: runner is nil")
	}
	res, err := c.Runner.Run(ctx, args, env)
	if err != nil || res.ExitCode != 0 {
		return res, fmt.Errorf("bw %s: %w", strings.Join(args, " "), commandErr(res, err))
	}
	return res, nil
}

func clearEnv(env []string) {
	for i := range env {
		env[i] = ""
	}
}

func commandErr(res Result, err error) error {
	msg := bytes.TrimSpace(res.Stderr)
	if len(msg) > 512 {
		msg = msg[:512]
	}
	if len(msg) == 0 {
		if err == nil {
			return fmt.Errorf("exit status %d", res.ExitCode)
		}
		return err
	}
	if err == nil {
		return fmt.Errorf("exit status %d: %s", res.ExitCode, msg)
	}
	return fmt.Errorf("%w: %s", err, msg)
}

func wipe(b []byte) {
	full := b[:cap(b)]
	for i := range full {
		full[i] = 0
	}
}

const itemTypeLogin = 1

func (c *CLI) readItems(ctx context.Context, session []byte, now time.Time) ([]Item, error) {
	// Folders first, so a folder failure does not have to decode item passwords.
	folders, err := c.listFolders(ctx, session)
	if err != nil {
		return nil, err
	}
	items, err := c.listRawItems(ctx, session)
	if err != nil {
		return nil, err
	}

	names := folderNames(folders)
	out := make([]Item, 0, len(items))
	for _, it := range items {
		item, ok, err := decodeItem(it, names, now)
		if err != nil {
			return nil, err
		}
		if ok {
			out = append(out, item)
		}
	}
	return out, nil
}

func decodeItem(it rawItem, names map[string]string, now time.Time) (Item, bool, error) {
	if it.Login == nil || it.Login.Password == "" {
		return Item{}, false, nil
	}
	changedRaw := ""
	if it.Login.PasswordRevisionDate != nil {
		changedRaw = *it.Login.PasswordRevisionDate
	}
	changed, err := passwordChanged(it.ID, changedRaw, it.CreationDate)
	if err != nil {
		return Item{}, false, err
	}
	return Item{
		ID:      it.ID,
		Name:    it.Name,
		Folder:  lookupFolder(folderID(it.FolderID), names),
		AgeDays: ageDays(changed, now),
	}, true, nil
}

// passwordChanged prefers login.passwordRevisionDate, then creationDate.
// revisionDate is when any field changed, so it is not an age source.
func passwordChanged(id, passwordRevision, creation string) (time.Time, error) {
	raw := strings.TrimSpace(passwordRevision)
	field := "passwordRevisionDate"
	if raw == "" {
		raw = strings.TrimSpace(creation)
		field = "creationDate"
	}
	if raw == "" {
		return time.Time{}, fmt.Errorf("bw: item %s: missing passwordRevisionDate and creationDate", id)
	}
	t, err := time.Parse(time.RFC3339Nano, raw)
	if err != nil {
		return time.Time{}, fmt.Errorf("bw: item %s: %s: %w", id, field, err)
	}
	return t, nil
}

func ageDays(changed, now time.Time) int {
	if !now.After(changed) {
		return 0
	}
	return int(now.Sub(changed) / (24 * time.Hour))
}

func folderNames(folders []folder) map[string]string {
	names := make(map[string]string, len(folders))
	for _, f := range folders {
		if f.ID == "" {
			continue
		}
		names[f.ID] = f.Name
	}
	return names
}

func lookupFolder(id string, names map[string]string) *string {
	if id == "" {
		return nil
	}
	name, ok := names[id]
	if !ok {
		return nil
	}
	return &name
}

func folderID(id *string) string {
	if id == nil {
		return ""
	}
	return *id
}

type folder struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type rawItem struct {
	ID           string  `json:"id"`
	Name         string  `json:"name"`
	FolderID     *string `json:"folderId"`
	Type         int     `json:"type"`
	DeletedDate  *string `json:"deletedDate"`
	RevisionDate string  `json:"revisionDate"`
	CreationDate string  `json:"creationDate"`
	Login        *struct {
		Password             string  `json:"password"`
		PasswordRevisionDate *string `json:"passwordRevisionDate"`
	} `json:"login"`
}
