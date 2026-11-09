// Package cache holds the public vault items served by the MCP tools.
// The slice is mutex-guarded. Concurrent misses are not coalesced; singleflight comes later.
package cache

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"bw-mcp/internal/bw"
	"bw-mcp/internal/secrets"
)

// ErrNotFound is returned when the cache has no item with the requested id.
var ErrNotFound = errors.New("item not found")

// Status is a cache snapshot plus the current Bitwarden CLI status.
// LastSync is zero when the cache has never been filled.
type Status struct {
	LastSync   time.Time
	AgeMinutes int
	Items      int
	BWStatus   string
}

// Cache is the prototype item store.
// It stores only public items. Passwords are zeroed before a refresh returns.
// A refresh opens the CLI, syncs, reads the vault model, and closes.
// The whole refresh is retried once. Individual bw calls are not retried.
type Cache struct {
	cli     *bw.CLI
	secrets secrets.Secrets
	now     func() time.Time

	// refreshMu keeps two bw sessions from interleaving.
	// It does not collapse concurrent callers into one sync.
	refreshMu sync.Mutex

	mu       sync.Mutex
	items    []bw.Item
	synced   bool
	lastSync time.Time
}

// New returns a cache that reads cli.
// A nil now function uses time.Now.
func New(cli *bw.CLI, secs secrets.Secrets, now func() time.Time) *Cache {
	if now == nil {
		now = time.Now
	}
	return &Cache{
		cli:     cli,
		secrets: secs,
		now:     now,
	}
}

// ListItems returns the cached public items.
// An empty cache (no successful sync yet) refreshes once.
// A successful sync that stored zero items does not refresh again.
func (c *Cache) ListItems(ctx context.Context) ([]bw.Item, error) {
	if items, ok := c.snapshot(); ok {
		return items, nil
	}
	if _, _, err := c.ForceSync(ctx); err != nil {
		return nil, err
	}
	items, ok := c.snapshot()
	if !ok {
		return nil, errors.New("cache: sync did not fill the cache")
	}
	return items, nil
}

// GetItem returns one cached item. It does not sync.
func (c *Cache) GetItem(id string) (bw.Item, error) {
	if id == "" {
		return bw.Item{}, errors.New("cache: item id is empty")
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, it := range c.items {
		if it.ID == id {
			return it, nil
		}
	}
	return bw.Item{}, fmt.Errorf("%w: %s", ErrNotFound, id)
}

// ForceSync opens the CLI, syncs, reads the vault model, and replaces the cached items.
// Each Login is zeroed before this returns, including on error.
func (c *Cache) ForceSync(ctx context.Context) (time.Time, int, error) {
	if c == nil || c.cli == nil {
		return time.Time{}, 0, errors.New("cache: cli is nil")
	}
	c.refreshMu.Lock()
	defer c.refreshMu.Unlock()

	var items []bw.Item
	err := c.withVault(ctx, func(v bw.Vault) error {
		items = bw.Items(v.Logins)
		return nil
	})
	if err != nil {
		return time.Time{}, 0, err
	}

	syncedAt := c.now()
	c.mu.Lock()
	c.items = items
	c.synced = true
	c.lastSync = syncedAt
	c.mu.Unlock()
	return syncedAt, len(items), nil
}

// Status reports cache age and the CLI status. It does not sync.
func (c *Cache) Status(ctx context.Context) (Status, error) {
	if c == nil || c.cli == nil {
		return Status{}, errors.New("cache: cli is nil")
	}
	st, err := c.cli.Status(ctx)
	if err != nil {
		return Status{}, err
	}
	if st.Status == "" {
		return Status{}, errors.New("cache: bw status is empty")
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return Status{
		LastSync:   c.lastSync,
		AgeMinutes: ageMinutes(c.lastSync, c.now()),
		Items:      len(c.items),
		BWStatus:   st.Status,
	}, nil
}

// withVault opens, syncs, reads, and closes.
// The whole operation is retried once. A new Open invalidates older session tokens,
// so a token is never reused across attempts. After success the CLI is locked.
// fn may be called twice if the first attempt fails. fn must not retain Login passwords.
func (c *Cache) withVault(ctx context.Context, fn func(bw.Vault) error) error {
	err := c.once(ctx, fn)
	if err == nil || ctx.Err() != nil {
		return err
	}
	return c.once(ctx, fn)
}

func (c *Cache) once(ctx context.Context, fn func(bw.Vault) error) (err error) {
	if err = ctx.Err(); err != nil {
		return err
	}
	if err = c.cli.Open(ctx, c.secrets); err != nil {
		return err
	}
	defer func() {
		if closeErr := c.cli.Close(ctx); closeErr != nil && err == nil {
			err = closeErr
		}
	}()
	if err = c.cli.Sync(ctx); err != nil {
		return err
	}
	v, err := c.cli.Read(ctx, c.now())
	if err != nil {
		return err
	}
	defer zeroLogins(v.Logins)
	return fn(v)
}

func (c *Cache) snapshot() ([]bw.Item, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.synced {
		return nil, false
	}
	return append([]bw.Item{}, c.items...), true
}

func ageMinutes(last, now time.Time) int {
	if last.IsZero() || !now.After(last) {
		return 0
	}
	return int(now.Sub(last) / time.Minute)
}

func zeroLogins(logins []bw.Login) {
	for i := range logins {
		logins[i].Zero()
	}
}
