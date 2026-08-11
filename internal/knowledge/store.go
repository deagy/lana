// Package knowledge implements Lana's local, file-backed knowledge store.
// It deliberately has no provider, network, or credential dependencies.
package knowledge

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode"

	"golang.org/x/sys/unix"
)

const (
	indexFileName       = "index.json"
	lockFileName        = ".index.lock"
	currentIndexVersion = 2
	defaultMaxFileBytes = int64(1 << 20) // 1 MiB keeps the on-disk index bounded.
	defaultSnippetBytes = 1200
	defaultMaxDocuments = 10_000
	maxSourceNameRunes  = 256
)

var allowedExtensions = map[string]struct{}{
	".cfg": {}, ".go": {}, ".ini": {}, ".json": {}, ".md": {}, ".toml": {},
	".txt": {}, ".yaml": {}, ".yml": {},
}

// Options controls bounded local storage and retrieval.
type Options struct {
	Dir             string
	MaxFileBytes    int64
	MaxDocuments    int
	MaxSnippetBytes int
}

// Store is a local JSON index. It is safe to create one for every command
// invocation; persistence is handled by atomic replacement of index.json.
type Store struct {
	dir             string
	maxFileBytes    int64
	maxDocuments    int
	maxSnippetBytes int
}

// New creates a store rooted at opts.Dir. The directory is not created until
// an ingest or deletion actually needs to persist a change.
func New(opts Options) (*Store, error) {
	if strings.TrimSpace(opts.Dir) == "" {
		return nil, fmt.Errorf("knowledge store path must not be empty")
	}
	dir, err := filepath.Abs(opts.Dir)
	if err != nil {
		return nil, fmt.Errorf("resolve knowledge store path: %w", err)
	}
	if opts.MaxFileBytes <= 0 {
		opts.MaxFileBytes = defaultMaxFileBytes
	}
	if opts.MaxSnippetBytes <= 0 {
		opts.MaxSnippetBytes = defaultSnippetBytes
	}
	if opts.MaxDocuments <= 0 {
		opts.MaxDocuments = defaultMaxDocuments
	}
	return &Store{dir: filepath.Clean(dir), maxFileBytes: opts.MaxFileBytes, maxDocuments: opts.MaxDocuments, maxSnippetBytes: opts.MaxSnippetBytes}, nil
}

// Dir returns the absolute local storage directory.
func (s *Store) Dir() string { return s.dir }

// Source is durable registration metadata for a local source tree or file.
type Source struct {
	ID         string    `json:"id"`
	Root       string    `json:"root"`
	Kind       string    `json:"kind"`
	Registered time.Time `json:"registered_at"`
	Updated    time.Time `json:"updated_at"`
}

// Document is one bounded, locally indexed source file. Content is retained
// to make the store self-contained even when a source disappears later.
type Document struct {
	ID          string    `json:"id"`
	Source      string    `json:"source"`
	Path        string    `json:"path"`
	Content     string    `json:"content"`
	ContentHash string    `json:"content_hash"`
	Size        int64     `json:"size"`
	ModifiedAt  time.Time `json:"modified_at"`
	IndexedAt   time.Time `json:"indexed_at"`
	Tags        []string  `json:"tags,omitempty"`
}

type index struct {
	Version int `json:"version"`
	// Revision increments for every successful mutation. It is state versioning,
	// not a schema version, and makes the persisted update order observable.
	Revision  uint64     `json:"revision"`
	Sources   []Source   `json:"sources"`
	Documents []Document `json:"documents"`
	// LegacyEntries preserves readability of the pre-local-store index format.
	// It is only populated while decoding a v1 index and is never written back.
	LegacyEntries []legacyEntry `json:"entries,omitempty"`
}

type legacyEntry struct {
	ID        string   `json:"id"`
	Source    string   `json:"source"`
	Content   string   `json:"content"`
	Tags      []string `json:"tags,omitempty"`
	IndexedAt string   `json:"indexed_at"`
}

// IngestResult precisely reports source-change detection.
type IngestResult struct {
	Source    string `json:"source"`
	Added     int    `json:"added"`
	Updated   int    `json:"updated"`
	Unchanged int    `json:"unchanged"`
	Removed   int    `json:"removed"`
	Skipped   int    `json:"skipped"`
}

// Ingest registers path as a source and upserts supported regular text files.
// Re-ingesting a directory removes documents that disappeared from that exact
// registered directory. Symlink files are never followed during a walk.
func (s *Store) Ingest(path, source string, tags []string) (IngestResult, error) {
	root, info, err := canonicalPath(path)
	if err != nil {
		return IngestResult{}, err
	}
	if source == "" {
		source = root
	}
	if err := validateSourceID(source); err != nil {
		return IngestResult{}, err
	}
	tags = normalizeTags(tags)

	candidates, skipped, err := s.collect(root, info.IsDir())
	if err != nil {
		return IngestResult{}, err
	}
	result := IngestResult{Source: source, Skipped: skipped}
	if len(candidates) > s.maxDocuments {
		result.Skipped += len(candidates) - s.maxDocuments
		candidates = candidates[:s.maxDocuments]
	}

	kind := "file"
	if info.IsDir() {
		kind = "directory"
	}
	err = s.update(func(idx *index) error {
		now := time.Now().UTC().Round(0)
		registered := -1
		for i := range idx.Sources {
			if idx.Sources[i].ID == source {
				registered = i
				break
			}
		}
		if registered >= 0 {
			existing := idx.Sources[registered]
			if existing.Root != root || existing.Kind != kind {
				return fmt.Errorf("source %q is already registered for %s", source, existing.Root)
			}
			idx.Sources[registered].Updated = now
		} else {
			idx.Sources = append(idx.Sources, Source{ID: source, Root: root, Kind: kind, Registered: now, Updated: now})
		}

		byID := make(map[string]int, len(idx.Documents))
		for i := range idx.Documents {
			byID[idx.Documents[i].ID] = i
		}
		seen := make(map[string]struct{}, len(candidates))
		for _, candidate := range candidates {
			doc, err := s.readDocument(source, candidate, tags)
			if err != nil {
				return err
			}
			seen[doc.ID] = struct{}{}
			if position, ok := byID[doc.ID]; ok {
				existing := idx.Documents[position]
				if existing.ContentHash == doc.ContentHash && sameStrings(existing.Tags, doc.Tags) {
					doc.IndexedAt = existing.IndexedAt
					result.Unchanged++
				} else {
					result.Updated++
				}
				idx.Documents[position] = doc
			} else {
				idx.Documents = append(idx.Documents, doc)
				byID[doc.ID] = len(idx.Documents) - 1
				result.Added++
			}
		}
		if info.IsDir() {
			kept := idx.Documents[:0]
			for _, doc := range idx.Documents {
				if doc.Source == source && isWithin(root, doc.Path) {
					if _, ok := seen[doc.ID]; !ok {
						result.Removed++
						continue
					}
				}
				kept = append(kept, doc)
			}
			idx.Documents = kept
		}
		return nil
	})
	if err != nil {
		return IngestResult{}, err
	}
	return result, nil
}

func (s *Store) collect(root string, directory bool) ([]string, int, error) {
	if !directory {
		if !ingestible(root) {
			return nil, 1, nil
		}
		info, err := os.Stat(root)
		if err != nil {
			return nil, 0, fmt.Errorf("stat source file: %w", err)
		}
		if info.Size() > s.maxFileBytes {
			return nil, 1, nil
		}
		return []string{root}, 0, nil
	}
	var paths []string
	skipped := 0
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == root {
			return nil
		}
		if d.Type()&os.ModeSymlink != 0 {
			skipped++
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			return nil
		}
		if !d.Type().IsRegular() || !ingestible(path) {
			skipped++
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		if info.Size() > s.maxFileBytes {
			skipped++
			return nil
		}
		paths = append(paths, path)
		return nil
	})
	if err != nil {
		return nil, skipped, fmt.Errorf("walk source: %w", err)
	}
	sort.Strings(paths)
	return paths, skipped, nil
}

func (s *Store) readDocument(source, path string, tags []string) (Document, error) {
	f, info, err := openRegularNoFollow(path)
	if err != nil {
		return Document{}, fmt.Errorf("open source file %q safely: %w", path, err)
	}
	defer f.Close()
	if info.Size() > s.maxFileBytes {
		return Document{}, fmt.Errorf("source file %q exceeds maximum size of %d bytes", path, s.maxFileBytes)
	}
	data, err := io.ReadAll(io.LimitReader(f, s.maxFileBytes+1))
	if err != nil {
		return Document{}, fmt.Errorf("read %q: %w", path, err)
	}
	if int64(len(data)) > s.maxFileBytes {
		return Document{}, fmt.Errorf("source file %q exceeds maximum size of %d bytes", path, s.maxFileBytes)
	}
	hash := hashBytes(data)
	now := time.Now().UTC().Round(0)
	return Document{ID: documentID(source, path), Source: source, Path: path, Content: string(data), ContentHash: hash, Size: int64(len(data)), ModifiedAt: info.ModTime().UTC().Round(0), IndexedAt: now, Tags: append([]string(nil), tags...)}, nil
}

// ListDocuments returns stable metadata order and never exposes mutable store
// state to callers.
func (s *Store) ListDocuments(source string) ([]Document, error) {
	idx, err := s.load()
	if err != nil {
		return nil, err
	}
	result := make([]Document, 0, len(idx.Documents))
	for _, doc := range idx.Documents {
		if source == "" || doc.Source == source {
			result = append(result, cloneDocument(doc))
		}
	}
	sortDocuments(result)
	return result, nil
}

// ListSources returns a deterministic snapshot of registrations.
func (s *Store) ListSources() ([]Source, error) {
	idx, err := s.load()
	if err != nil {
		return nil, err
	}
	result := append([]Source(nil), idx.Sources...)
	sort.Slice(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	return result, nil
}

// Citation identifies exactly the stored evidence used for a search result.
type Citation struct {
	Source      string `json:"source"`
	Path        string `json:"path"`
	DocumentID  string `json:"document_id"`
	ChunkID     string `json:"chunk_id"`
	ContentHash string `json:"content_hash"`
}

// SearchOptions constrains deterministic local retrieval.
type SearchOptions struct {
	Top    int
	Source string
	Tags   []string
}

// Result is a bounded retrieval passage plus its citation.
type Result struct {
	Score    int      `json:"score"`
	Content  string   `json:"content"`
	Tags     []string `json:"tags,omitempty"`
	Citation Citation `json:"citation"`
}

// Search performs deterministic case-insensitive token matching. Ties sort by
// source, path, then document ID so identical stores always return the same
// response.
func (s *Store) Search(query string, opts SearchOptions) ([]Result, error) {
	terms := searchTerms(query)
	if len(terms) == 0 {
		return nil, fmt.Errorf("search query must contain a word")
	}
	if opts.Top <= 0 {
		opts.Top = 10
	}
	if opts.Top > 100 {
		return nil, fmt.Errorf("top must be between 1 and 100")
	}
	idx, err := s.load()
	if err != nil {
		return nil, err
	}
	tags := normalizeTags(opts.Tags)
	results := make([]Result, 0)
	for _, doc := range idx.Documents {
		if opts.Source != "" && doc.Source != opts.Source {
			continue
		}
		if !containsAllTags(doc.Tags, tags) {
			continue
		}
		score := score(doc, terms)
		if score == 0 {
			continue
		}
		results = append(results, Result{Score: score, Content: truncateUTF8(doc.Content, s.maxSnippetBytes), Tags: append([]string(nil), doc.Tags...), Citation: Citation{Source: doc.Source, Path: doc.Path, DocumentID: doc.ID, ChunkID: doc.ID, ContentHash: doc.ContentHash}})
	}
	sort.Slice(results, func(i, j int) bool {
		if results[i].Score != results[j].Score {
			return results[i].Score > results[j].Score
		}
		a, b := results[i].Citation, results[j].Citation
		if a.Source != b.Source {
			return a.Source < b.Source
		}
		if a.Path != b.Path {
			return a.Path < b.Path
		}
		return a.DocumentID < b.DocumentID
	})
	if len(results) > opts.Top {
		results = results[:opts.Top]
	}
	return results, nil
}

// RemoveDocument deletes a single indexed document. It is deliberately
// separate from source removal so callers cannot accidentally purge a source.
func (s *Store) RemoveDocument(id string) error {
	return s.update(func(idx *index) error {
		for i, doc := range idx.Documents {
			if doc.ID == id {
				idx.Documents = append(idx.Documents[:i], idx.Documents[i+1:]...)
				return nil
			}
		}
		return fmt.Errorf("knowledge document not found: %s", id)
	})
}

// RemoveSource deletes a registration and all of its stored documents.
func (s *Store) RemoveSource(source string) (int, error) {
	removed := 0
	err := s.update(func(idx *index) error {
		found := false
		sources := idx.Sources[:0]
		for _, item := range idx.Sources {
			if item.ID == source {
				found = true
				continue
			}
			sources = append(sources, item)
		}
		if !found {
			return fmt.Errorf("knowledge source not found: %s", source)
		}
		idx.Sources = sources
		kept := idx.Documents[:0]
		for _, doc := range idx.Documents {
			if doc.Source == source {
				removed++
				continue
			}
			kept = append(kept, doc)
		}
		idx.Documents = kept
		return nil
	})
	return removed, err
}

func (s *Store) load() (*index, error) {
	dir, err := openDirectoryNoFollow(s.dir, false)
	if errors.Is(err, os.ErrNotExist) {
		return &index{Version: currentIndexVersion}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("open knowledge store safely: %w", err)
	}
	defer dir.Close()
	return loadIndex(dir)
}

// update serializes the complete read-modify-write operation across processes.
// Advisory locking is appropriate here because every Lana store mutation uses
// this path; a non-cooperating process is intentionally outside that contract.
func (s *Store) update(mutate func(*index) error) error {
	dir, err := openDirectoryNoFollow(s.dir, true)
	if err != nil {
		return fmt.Errorf("open knowledge store safely: %w", err)
	}
	defer dir.Close()
	if err := unix.Fchmod(int(dir.Fd()), 0o700); err != nil {
		return fmt.Errorf("secure knowledge store: %w", err)
	}
	lockFD, err := unix.Openat(int(dir.Fd()), lockFileName, unix.O_RDWR|unix.O_CREAT|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0o600)
	if err != nil {
		return fmt.Errorf("open knowledge store lock: %w", err)
	}
	lock := os.NewFile(uintptr(lockFD), lockFileName)
	defer lock.Close()
	if err := unix.Fchmod(lockFD, 0o600); err != nil {
		return fmt.Errorf("secure knowledge store lock: %w", err)
	}
	if err := unix.Flock(lockFD, unix.LOCK_EX); err != nil {
		return fmt.Errorf("lock knowledge store: %w", err)
	}
	defer unix.Flock(lockFD, unix.LOCK_UN)
	idx, err := loadIndex(dir)
	if err != nil {
		return err
	}
	if err := mutate(idx); err != nil {
		return err
	}
	idx.Revision++
	return saveIndex(dir, idx)
}

func loadIndex(dir *os.File) (*index, error) {
	fd, err := unix.Openat(int(dir.Fd()), indexFileName, unix.O_RDONLY|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0)
	if errors.Is(err, os.ErrNotExist) {
		return &index{Version: currentIndexVersion}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read knowledge index: %w", err)
	}
	f := os.NewFile(uintptr(fd), indexFileName)
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return nil, fmt.Errorf("stat knowledge index: %w", err)
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("knowledge index is not a regular file")
	}
	data, err := io.ReadAll(f)
	if err != nil {
		return nil, fmt.Errorf("read knowledge index: %w", err)
	}
	var idx index
	if err := json.Unmarshal(data, &idx); err != nil {
		return nil, fmt.Errorf("parse knowledge index: %w", err)
	}
	if idx.Version > currentIndexVersion {
		return nil, fmt.Errorf("knowledge index version %d is newer than this Lana version", idx.Version)
	}
	if idx.Version == 0 {
		idx.Version = currentIndexVersion
	}
	if len(idx.Documents) == 0 && len(idx.LegacyEntries) > 0 {
		for _, legacy := range idx.LegacyEntries {
			path := legacy.Source
			id := legacy.ID
			if id == "" {
				id = documentID(legacy.Source, path)
			}
			indexedAt, _ := time.Parse(time.RFC3339, legacy.IndexedAt)
			idx.Documents = append(idx.Documents, Document{ID: id, Source: legacy.Source, Path: path, Content: legacy.Content, ContentHash: hashBytes([]byte(legacy.Content)), Size: int64(len(legacy.Content)), IndexedAt: indexedAt.UTC(), Tags: normalizeTags(legacy.Tags)})
		}
		idx.LegacyEntries = nil
	}
	for i := range idx.Documents {
		doc := &idx.Documents[i]
		if doc.Path == "" {
			doc.Path = doc.Source
		}
		if doc.ID == "" {
			doc.ID = documentID(doc.Source, doc.Path)
		}
		if doc.ContentHash == "" {
			doc.ContentHash = hashBytes([]byte(doc.Content))
		}
		doc.Tags = normalizeTags(doc.Tags)
	}
	return &idx, nil
}

func saveIndex(dir *os.File, idx *index) error {
	idx.Version = currentIndexVersion
	sort.Slice(idx.Sources, func(i, j int) bool { return idx.Sources[i].ID < idx.Sources[j].ID })
	sortDocuments(idx.Documents)
	data, err := json.MarshalIndent(idx, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal knowledge index: %w", err)
	}
	name := fmt.Sprintf(".index-%d-%d.tmp", os.Getpid(), time.Now().UnixNano())
	fd, err := unix.Openat(int(dir.Fd()), name, unix.O_WRONLY|unix.O_CREAT|unix.O_EXCL|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0o600)
	if err != nil {
		return fmt.Errorf("create temporary knowledge index: %w", err)
	}
	defer unix.Unlinkat(int(dir.Fd()), name, 0)
	temp := os.NewFile(uintptr(fd), name)
	if err := unix.Fchmod(fd, 0o600); err != nil {
		temp.Close()
		return fmt.Errorf("secure temporary knowledge index: %w", err)
	}
	if _, err := temp.Write(data); err != nil {
		temp.Close()
		return fmt.Errorf("write knowledge index: %w", err)
	}
	if err := temp.Sync(); err != nil {
		temp.Close()
		return fmt.Errorf("sync knowledge index: %w", err)
	}
	if err := temp.Close(); err != nil {
		return fmt.Errorf("close knowledge index: %w", err)
	}
	if err := unix.Renameat(int(dir.Fd()), name, int(dir.Fd()), indexFileName); err != nil {
		return fmt.Errorf("replace knowledge index: %w", err)
	}
	if err := unix.Fsync(int(dir.Fd())); err != nil {
		return fmt.Errorf("sync knowledge store directory: %w", err)
	}
	return nil
}

func canonicalPath(path string) (string, os.FileInfo, error) {
	if strings.TrimSpace(path) == "" {
		return "", nil, fmt.Errorf("source path must not be empty")
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", nil, fmt.Errorf("resolve source path: %w", err)
	}
	resolved, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return "", nil, fmt.Errorf("resolve source path: %w", err)
	}
	info, err := safePathInfo(resolved)
	if err != nil {
		return "", nil, fmt.Errorf("open source path safely: %w", err)
	}
	if !info.IsDir() && !info.Mode().IsRegular() {
		return "", nil, fmt.Errorf("source path must be a regular file or directory")
	}
	return filepath.Clean(resolved), info, nil
}

// openDirectoryNoFollow walks from the filesystem root using directory file
// descriptors. Each component is opened with O_NOFOLLOW so a path component
// swapped for a symlink cannot redirect reads or writes outside the intended
// directory. On platforms/filesystems that cannot provide that property it
// fails rather than silently falling back to path-based access.
func openDirectoryNoFollow(path string, create bool) (*os.File, error) {
	if !filepath.IsAbs(path) {
		return nil, fmt.Errorf("path is not absolute")
	}
	fd, err := unix.Open(string(filepath.Separator), unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC, 0)
	if err != nil {
		return nil, err
	}
	current := os.NewFile(uintptr(fd), string(filepath.Separator))
	for _, component := range strings.Split(strings.TrimPrefix(filepath.Clean(path), string(filepath.Separator)), string(filepath.Separator)) {
		if component == "" || component == "." {
			continue
		}
		nextFD, openErr := unix.Openat(int(current.Fd()), component, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0)
		if errors.Is(openErr, unix.ENOENT) && create {
			if mkdirErr := unix.Mkdirat(int(current.Fd()), component, 0o700); mkdirErr != nil && !errors.Is(mkdirErr, unix.EEXIST) {
				current.Close()
				return nil, mkdirErr
			}
			nextFD, openErr = unix.Openat(int(current.Fd()), component, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0)
		}
		if openErr != nil {
			current.Close()
			return nil, openErr
		}
		next := os.NewFile(uintptr(nextFD), component)
		current.Close()
		current = next
	}
	return current, nil
}

func safePathInfo(path string) (os.FileInfo, error) {
	if dir, err := openDirectoryNoFollow(path, false); err == nil {
		defer dir.Close()
		return dir.Stat()
	}
	file, info, err := openRegularNoFollow(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return info, nil
}

func openRegularNoFollow(path string) (*os.File, os.FileInfo, error) {
	parent, err := openDirectoryNoFollow(filepath.Dir(path), false)
	if err != nil {
		return nil, nil, err
	}
	defer parent.Close()
	fd, err := unix.Openat(int(parent.Fd()), filepath.Base(path), unix.O_RDONLY|unix.O_NONBLOCK|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0)
	if err != nil {
		return nil, nil, err
	}
	file := os.NewFile(uintptr(fd), filepath.Base(path))
	info, err := file.Stat()
	if err != nil {
		file.Close()
		return nil, nil, err
	}
	if !info.Mode().IsRegular() {
		file.Close()
		return nil, nil, fmt.Errorf("not a regular file")
	}
	return file, info, nil
}

func validateSourceID(source string) error {
	if strings.TrimSpace(source) == "" {
		return fmt.Errorf("source must not be empty")
	}
	if len([]rune(source)) > maxSourceNameRunes {
		return fmt.Errorf("source is longer than %d characters", maxSourceNameRunes)
	}
	for _, r := range source {
		if unicode.IsControl(r) {
			return fmt.Errorf("source contains a control character")
		}
	}
	return nil
}

func ingestible(path string) bool {
	_, ok := allowedExtensions[strings.ToLower(filepath.Ext(path))]
	return ok
}
func documentID(source, path string) string {
	return "doc-" + hashBytes([]byte(source + "\x00" + path))[:16]
}
func hashBytes(data []byte) string { sum := sha256.Sum256(data); return hex.EncodeToString(sum[:]) }
func normalizeTags(tags []string) []string {
	set := map[string]struct{}{}
	for _, tag := range tags {
		if tag = strings.ToLower(strings.TrimSpace(tag)); tag != "" {
			set[tag] = struct{}{}
		}
	}
	result := make([]string, 0, len(set))
	for tag := range set {
		result = append(result, tag)
	}
	sort.Strings(result)
	return result
}
func sameStrings(a, b []string) bool {
	return len(a) == len(b) && func() bool {
		for i := range a {
			if a[i] != b[i] {
				return false
			}
		}
		return true
	}()
}
func containsAllTags(have, wanted []string) bool {
	for _, wantedTag := range wanted {
		found := false
		for _, tag := range have {
			if tag == wantedTag {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}
func isWithin(root, path string) bool {
	relative, err := filepath.Rel(root, path)
	return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}
func cloneDocument(doc Document) Document { doc.Tags = append([]string(nil), doc.Tags...); return doc }
func sortDocuments(docs []Document) {
	sort.Slice(docs, func(i, j int) bool {
		if docs[i].Source != docs[j].Source {
			return docs[i].Source < docs[j].Source
		}
		if docs[i].Path != docs[j].Path {
			return docs[i].Path < docs[j].Path
		}
		return docs[i].ID < docs[j].ID
	})
}
func searchTerms(query string) []string {
	return normalizeTags(strings.FieldsFunc(strings.ToLower(query), func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsNumber(r) }))
}
func score(doc Document, terms []string) int {
	content, source, path := strings.ToLower(doc.Content), strings.ToLower(doc.Source), strings.ToLower(doc.Path)
	total := 0
	for _, term := range terms {
		total += 10 * strings.Count(content, term)
		total += 5 * strings.Count(source, term)
		total += 3 * strings.Count(path, term)
		for _, tag := range doc.Tags {
			total += 4 * strings.Count(tag, term)
		}
	}
	return total
}
func truncateUTF8(value string, maximum int) string {
	if len(value) <= maximum {
		return value
	}
	end := maximum
	for end > 0 && (value[end]&0xc0) == 0x80 {
		end--
	}
	return value[:end] + "…"
}
