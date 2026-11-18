package cache

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestConcurrentMissesShareOneRefresh(t *testing.T) {
	const n = 8
	release := make(chan struct{})
	runner := newScriptRunner(sampleItems())
	runner.holdSync = release
	now := time.Date(2024, 1, 31, 0, 0, 0, 0, time.UTC)
	store := New(newCLI(runner), testSecrets(), func() time.Time { return now })

	entered := make(chan struct{}, n)
	store.entered = func() { entered <- struct{}{} }

	var wg sync.WaitGroup
	errCh := make(chan error, n)
	wg.Add(n)
	for range n {
		go func() {
			defer wg.Done()
			_, err := store.ListItems(context.Background())
			errCh <- err
		}()
	}
	for range n {
		<-entered
	}
	close(release)
	wg.Wait()
	close(errCh)
	for err := range errCh {
		if err != nil {
			t.Fatal(err)
		}
	}
	if syncs := strings.Count(strings.Join(runner.calls, ","), "sync"); syncs != 1 {
		t.Fatalf("syncs = %d, want 1; calls = %#v", syncs, runner.calls)
	}
	items, err := store.ListItems(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 || items[0].ID != "github" {
		t.Fatalf("items = %+v", items)
	}
	if syncs := strings.Count(strings.Join(runner.calls, ","), "sync"); syncs != 1 {
		t.Fatalf("list after refresh synced again: %#v", runner.calls)
	}
	assertNoSecrets(t, mustJSON(t, items))
}

func TestEmptyCacheStillRefreshes(t *testing.T) {
	runner := newScriptRunner(sampleItems())
	store := New(newCLI(runner), testSecrets(), time.Now)
	items, err := store.ListItems(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 {
		t.Fatalf("items = %d", len(items))
	}
	if _, err := store.ListItems(context.Background()); err != nil {
		t.Fatal(err)
	}
	if syncs := strings.Count(strings.Join(runner.calls, ","), "sync"); syncs != 1 {
		t.Fatalf("syncs = %d, want 1; calls = %#v", syncs, runner.calls)
	}
}

func TestForceSyncStillRefreshes(t *testing.T) {
	runner := newScriptRunner(sampleItems())
	store := New(newCLI(runner), testSecrets(), time.Now)
	if _, _, err := store.ForceSync(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, _, err := store.ForceSync(context.Background()); err != nil {
		t.Fatal(err)
	}
	if syncs := strings.Count(strings.Join(runner.calls, ","), "sync"); syncs != 2 {
		t.Fatalf("syncs = %d, want 2; calls = %#v", syncs, runner.calls)
	}
}
