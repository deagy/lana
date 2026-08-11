package knowledge

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestIngestUpsertsDetectsChangesAndRemovesMissingFiles(t *testing.T) {
	sourceDir := t.TempDir()
	writeSource(t, filepath.Join(sourceDir, "one.md"), "alpha durable local knowledge")
	writeSource(t, filepath.Join(sourceDir, "two.txt"), "beta fixture")
	writeSource(t, filepath.Join(sourceDir, "ignored.bin"), "not text")
	store := testStore(t, 1024)

	first, err := store.Ingest(sourceDir, "fixture", []string{"Docs", "docs", "local"})
	if err != nil {
		t.Fatal(err)
	}
	if first.Added != 2 || first.Skipped != 1 {
		t.Fatalf("first ingest = %#v", first)
	}
	docs, err := store.ListDocuments("fixture")
	if err != nil {
		t.Fatal(err)
	}
	if len(docs) != 2 || strings.Join(docs[0].Tags, ",") != "docs,local" {
		t.Fatalf("documents = %#v", docs)
	}

	second, err := store.Ingest(sourceDir, "fixture", []string{"local", "docs"})
	if err != nil {
		t.Fatal(err)
	}
	if second.Unchanged != 2 || second.Added != 0 || second.Updated != 0 {
		t.Fatalf("second ingest = %#v", second)
	}

	writeSource(t, filepath.Join(sourceDir, "one.md"), "alpha changed knowledge")
	if err := os.Remove(filepath.Join(sourceDir, "two.txt")); err != nil {
		t.Fatal(err)
	}
	third, err := store.Ingest(sourceDir, "fixture", []string{"docs", "local"})
	if err != nil {
		t.Fatal(err)
	}
	if third.Updated != 1 || third.Removed != 1 {
		t.Fatalf("third ingest = %#v", third)
	}
	docs, err = store.ListDocuments("fixture")
	if err != nil || len(docs) != 1 || !strings.Contains(docs[0].Content, "changed") {
		t.Fatalf("post-change docs = %#v, %v", docs, err)
	}
}

func TestIngestBoundsFilesAndDoesNotFollowWalkedSymlinks(t *testing.T) {
	sourceDir := t.TempDir()
	writeSource(t, filepath.Join(sourceDir, "small.txt"), "small")
	writeSource(t, filepath.Join(sourceDir, "large.txt"), strings.Repeat("x", 65))
	outside := filepath.Join(t.TempDir(), "outside.txt")
	writeSource(t, outside, "outside")
	if err := os.Symlink(outside, filepath.Join(sourceDir, "linked.txt")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	store := testStore(t, 64)
	result, err := store.Ingest(sourceDir, "safe", nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.Added != 1 || result.Skipped != 2 {
		t.Fatalf("bounded ingest = %#v", result)
	}
	docs, err := store.ListDocuments("safe")
	if err != nil || len(docs) != 1 || docs[0].Path != filepath.Join(sourceDir, "small.txt") {
		t.Fatalf("docs = %#v, %v", docs, err)
	}
}

func TestIngestBoundsDocumentCountDeterministically(t *testing.T) {
	sourceDir := t.TempDir()
	writeSource(t, filepath.Join(sourceDir, "c.txt"), "c")
	writeSource(t, filepath.Join(sourceDir, "a.txt"), "a")
	writeSource(t, filepath.Join(sourceDir, "b.txt"), "b")
	store := testStore(t, 1024)
	store.maxDocuments = 2
	result, err := store.Ingest(sourceDir, "bounded", nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.Added != 2 || result.Skipped != 1 {
		t.Fatalf("bounded count ingest = %#v", result)
	}
	documents, err := store.ListDocuments("bounded")
	if err != nil || len(documents) != 2 || !strings.HasSuffix(documents[0].Path, "a.txt") || !strings.HasSuffix(documents[1].Path, "b.txt") {
		t.Fatalf("bounded documents = %#v, %v", documents, err)
	}
}

func TestSearchIsDeterministicBoundedAndCited(t *testing.T) {
	sourceDir := t.TempDir()
	writeSource(t, filepath.Join(sourceDir, "b.md"), strings.Repeat("needle ", 30))
	writeSource(t, filepath.Join(sourceDir, "a.md"), "needle in a haystack")
	store := testStore(t, 4096)
	store.maxSnippetBytes = 24
	if _, err := store.Ingest(sourceDir, "notes", []string{"local"}); err != nil {
		t.Fatal(err)
	}
	results, err := store.Search("needle", SearchOptions{Top: 10, Tags: []string{"LOCAL"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 {
		t.Fatalf("result count = %d", len(results))
	}
	if results[0].Score <= results[1].Score {
		t.Fatalf("results are not score ordered: %#v", results)
	}
	if results[0].Citation.Source != "notes" || results[0].Citation.DocumentID == "" || results[0].Citation.ContentHash == "" || results[0].Citation.ChunkID != results[0].Citation.DocumentID {
		t.Fatalf("citation = %#v", results[0].Citation)
	}
	if len(results[0].Content) > 27 || !strings.HasSuffix(results[0].Content, "…") {
		t.Fatalf("unbounded content = %q", results[0].Content)
	}
	repeated, err := store.Search("needle", SearchOptions{Top: 10})
	if err != nil {
		t.Fatal(err)
	}
	if results[0].Citation.DocumentID != repeated[0].Citation.DocumentID {
		t.Fatalf("search order changed: %#v vs %#v", results, repeated)
	}
}

func TestSourceRegistrationDeletionAndAtomicDurableIndex(t *testing.T) {
	sourceA := t.TempDir()
	sourceB := t.TempDir()
	writeSource(t, filepath.Join(sourceA, "one.txt"), "one")
	store := testStore(t, 1024)
	if _, err := store.Ingest(sourceA, "named", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Ingest(sourceB, "named", nil); err == nil {
		t.Fatal("expected source re-registration to fail")
	}
	sources, err := store.ListSources()
	if err != nil || len(sources) != 1 || sources[0].Root != sourceA {
		t.Fatalf("sources = %#v, %v", sources, err)
	}
	removed, err := store.RemoveSource("named")
	if err != nil || removed != 1 {
		t.Fatalf("remove source = %d, %v", removed, err)
	}
	docs, err := store.ListDocuments("")
	if err != nil || len(docs) != 0 {
		t.Fatalf("documents after removal = %#v, %v", docs, err)
	}
	info, err := os.Stat(filepath.Join(store.Dir(), indexFileName))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("index permissions = %o, want 600", info.Mode().Perm())
	}
	dirInfo, err := os.Stat(store.Dir())
	if err != nil {
		t.Fatal(err)
	}
	if dirInfo.Mode().Perm() != 0o700 {
		t.Fatalf("store permissions = %o, want 700", dirInfo.Mode().Perm())
	}
}

func TestLegacyIndexRemainsReadable(t *testing.T) {
	dir := t.TempDir()
	data := []byte(`{"version":1,"entries":[{"id":"entry-0001","source":"fixture.md","content":"legacy knowledge","tags":["legacy"],"indexed_at":"2026-08-11T00:00:00Z"}]}`)
	if err := os.WriteFile(filepath.Join(dir, indexFileName), data, 0o600); err != nil {
		t.Fatal(err)
	}
	store, err := New(Options{Dir: dir})
	if err != nil {
		t.Fatal(err)
	}
	results, err := store.Search("legacy", SearchOptions{Top: 1})
	if err != nil || len(results) != 1 || results[0].Citation.DocumentID != "entry-0001" {
		t.Fatalf("legacy search = %#v, %v", results, err)
	}
	if _, err := store.Ingest(t.TempDir(), "empty", nil); err != nil {
		t.Fatal(err)
	}
	persisted, err := os.ReadFile(filepath.Join(dir, indexFileName))
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(persisted, &decoded); err != nil {
		t.Fatal(err)
	}
	if _, old := decoded["entries"]; old {
		t.Fatalf("legacy entries were retained after write: %s", persisted)
	}
}

func TestReadDocumentRejectsSymlinkSwappedAfterCollection(t *testing.T) {
	sourceDir := t.TempDir()
	path := filepath.Join(sourceDir, "entry.txt")
	writeSource(t, path, "safe")
	store := testStore(t, 1024)
	candidates, _, err := store.collect(sourceDir, true)
	if err != nil || len(candidates) != 1 {
		t.Fatalf("collect = %#v, %v", candidates, err)
	}
	outside := filepath.Join(t.TempDir(), "outside.txt")
	writeSource(t, outside, "must not be read")
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, path); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if _, err := store.readDocument("safe", candidates[0], nil); err == nil {
		t.Fatal("readDocument accepted a source file replaced by a symlink")
	}
}

func TestIndexSymlinkIsRejected(t *testing.T) {
	dir := t.TempDir()
	store, err := New(Options{Dir: dir})
	if err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(t.TempDir(), "not-an-index.json")
	writeSource(t, target, `{"version":2,"documents":[]}`)
	if err := os.Symlink(target, filepath.Join(dir, indexFileName)); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if _, err := store.ListDocuments(""); err == nil {
		t.Fatal("reading an index symlink unexpectedly succeeded")
	}
}

// TestStoreHelperProcess is invoked in child test binaries below. A separate
// process is essential: the lock protects command invocations, not just
// goroutines that happen to share a Store.
func TestStoreHelperProcess(t *testing.T) {
	if os.Getenv("LANA_KNOWLEDGE_HELPER") != "ingest" {
		return
	}
	store, err := New(Options{Dir: os.Getenv("LANA_KNOWLEDGE_STORE")})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Ingest(os.Getenv("LANA_KNOWLEDGE_SOURCE_PATH"), os.Getenv("LANA_KNOWLEDGE_SOURCE_ID"), nil); err != nil {
		t.Fatal(err)
	}
}

func TestConcurrentProcessIngestsDoNotLoseUpdates(t *testing.T) {
	root := t.TempDir()
	storeDir := filepath.Join(root, "store")
	const workers = 10
	const documentsPerWorker = 40
	commands := make([]*exec.Cmd, 0, workers)
	for worker := 0; worker < workers; worker++ {
		sourceDir := filepath.Join(root, "source", string(rune('a'+worker)))
		if err := os.MkdirAll(sourceDir, 0o700); err != nil {
			t.Fatal(err)
		}
		for document := 0; document < documentsPerWorker; document++ {
			writeSource(t, filepath.Join(sourceDir, fmt.Sprintf("%03d.txt", document)), fmt.Sprintf("worker %d document %d", worker, document))
		}
		command := exec.Command(os.Args[0], "-test.run=^TestStoreHelperProcess$")
		command.Env = append(os.Environ(),
			"LANA_KNOWLEDGE_HELPER=ingest",
			"LANA_KNOWLEDGE_STORE="+storeDir,
			"LANA_KNOWLEDGE_SOURCE_PATH="+sourceDir,
			fmt.Sprintf("LANA_KNOWLEDGE_SOURCE_ID=worker-%d", worker),
		)
		commands = append(commands, command)
	}
	var wait sync.WaitGroup
	errs := make(chan error, len(commands))
	for _, command := range commands {
		wait.Add(1)
		go func(command *exec.Cmd) {
			defer wait.Done()
			errs <- command.Run()
		}(command)
	}
	wait.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("concurrent child ingest: %v", err)
		}
	}
	store, err := New(Options{Dir: storeDir})
	if err != nil {
		t.Fatal(err)
	}
	sources, err := store.ListSources()
	if err != nil || len(sources) != workers {
		t.Fatalf("sources after concurrent ingests = %d, %v", len(sources), err)
	}
	documents, err := store.ListDocuments("")
	if err != nil || len(documents) != workers*documentsPerWorker {
		t.Fatalf("documents after concurrent ingests = %d, %v", len(documents), err)
	}
	data, err := os.ReadFile(filepath.Join(storeDir, indexFileName))
	if err != nil {
		t.Fatal(err)
	}
	var persisted index
	if err := json.Unmarshal(data, &persisted); err != nil {
		t.Fatal(err)
	}
	if persisted.Revision != workers {
		t.Fatalf("revision = %d, want %d", persisted.Revision, workers)
	}
}

func testStore(t *testing.T, maxBytes int64) *Store {
	t.Helper()
	store, err := New(Options{Dir: filepath.Join(t.TempDir(), "store"), MaxFileBytes: maxBytes})
	if err != nil {
		t.Fatal(err)
	}
	return store
}

func writeSource(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}
