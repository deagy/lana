package session

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestStoreAppendListLoadAndFork(t *testing.T) {
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if _, err := store.Create(ctx, "parent", map[string]string{"model": "test"}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Append(ctx, "parent", "message", map[string]string{"content": "hello"}); err != nil {
		t.Fatal(err)
	}
	loaded, err := store.Load(ctx, "parent")
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.Records) != 2 || loaded.Metadata["model"] != "test" {
		t.Fatalf("loaded = %#v", loaded)
	}
	fork, err := store.Fork(ctx, "parent", "child")
	if err != nil {
		t.Fatal(err)
	}
	if fork.ParentID != "parent" || len(fork.Records) != 3 {
		t.Fatalf("fork = %#v", fork)
	}
	items, err := store.List(ctx)
	if err != nil || len(items) != 2 {
		t.Fatalf("items=%#v err=%v", items, err)
	}
}

func TestStoreRejectsCompleteCorruptFinalRecord(t *testing.T) {
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if _, err := store.Create(ctx, "corrupt", nil); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(store.Root(), "corrupt.jsonl"), append(mustRead(t, filepath.Join(store.Root(), "corrupt.jsonl")), []byte(`{"schema_version":`+"\n")...), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Load(ctx, "corrupt"); err == nil || !strings.Contains(err.Error(), "decode session") {
		t.Fatalf("complete corrupt record must fail, err=%v", err)
	}
	if _, err := store.Recover(ctx, "corrupt"); err == nil {
		t.Fatal("recovery must not erase a complete corrupt record")
	}
}

func TestStoreAppendCancellationFsyncAndRedaction(t *testing.T) {
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if _, err := store.Create(ctx, "errors", map[string]string{"api_token": "secret"}); err != nil {
		t.Fatal(err)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := store.Append(cancelled, "errors", "message", map[string]string{"ok": "yes"}); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled append error = %v", err)
	}
	store.syncFile = func(*os.File) error { return errors.New("simulated fsync failure") }
	if _, err := store.Append(ctx, "errors", "message", map[string]any{"payload": map[string]string{"refresh_token": "secret"}}); err == nil || !strings.Contains(err.Error(), "sync") {
		t.Fatalf("fsync error = %v", err)
	}
	contents := string(mustRead(t, filepath.Join(store.Root(), "errors.jsonl")))
	if strings.Contains(contents, "secret") {
		t.Fatalf("secret leaked to session: %s", contents)
	}
}

func TestStoreAssignsSequentialRecordsUnderConcurrency(t *testing.T) {
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	store.now = func() time.Time { return time.Date(2026, 8, 10, 0, 0, 0, 0, time.UTC) }
	ctx := context.Background()
	if _, err := store.Create(ctx, "concurrent", nil); err != nil {
		t.Fatal(err)
	}
	const writers = 24
	var group sync.WaitGroup
	group.Add(writers)
	errs := make(chan error, writers)
	for index := 0; index < writers; index++ {
		go func(index int) {
			defer group.Done()
			_, err := store.Append(ctx, "concurrent", "message", map[string]int{"index": index})
			errs <- err
		}(index)
	}
	group.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	loaded, err := store.Load(ctx, "concurrent")
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.Records) != writers+1 {
		t.Fatalf("records=%d", len(loaded.Records))
	}
	for index, record := range loaded.Records {
		if record.Sequence != int64(index+1) || !record.At.Equal(store.now()) {
			t.Fatalf("record %d = %#v", index, record)
		}
	}
}

func mustRead(t *testing.T, path string) []byte {
	t.Helper()
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return contents
}

func TestStoreRecoveryTrimsOnlyTornFinalLine(t *testing.T) {
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if _, err := store.Create(ctx, "crashed", nil); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(store.Root(), "crashed.jsonl")
	file, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.WriteString(`{"schema_version":1`); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	loaded, err := store.Load(ctx, "crashed")
	if err != nil || !loaded.Recovered {
		t.Fatalf("loaded=%#v err=%v", loaded, err)
	}
	recovered, err := store.Recover(ctx, "crashed")
	if err != nil || recovered.Recovered {
		t.Fatalf("recovered=%#v err=%v", recovered, err)
	}
	if _, err := store.Append(ctx, "crashed", "message", map[string]string{"ok": "yes"}); err != nil {
		t.Fatal(err)
	}
}

func TestStoreRejectsTraversalID(t *testing.T) {
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Create(context.Background(), "../escape", nil); err == nil {
		t.Fatal("expected invalid id")
	}
}
