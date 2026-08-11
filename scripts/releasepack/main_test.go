package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestPackIsDeterministicAndIncludesSourceFiles(t *testing.T) {
	t.Parallel()

	source := t.TempDir()
	if err := os.Mkdir(filepath.Join(source, "completions"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "lana"), []byte("binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "completions", "lana.bash"), []byte("completion"), 0o644); err != nil {
		t.Fatal(err)
	}

	first := filepath.Join(t.TempDir(), "first.tar.gz")
	second := filepath.Join(t.TempDir(), "second.tar.gz")
	epoch := time.Unix(1_735_689_600, 0).UTC()
	if err := pack(source, first, epoch); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(filepath.Join(source, "lana"), time.Now(), time.Now()); err != nil {
		t.Fatal(err)
	}
	if err := pack(source, second, epoch); err != nil {
		t.Fatal(err)
	}

	firstContents, err := os.ReadFile(first)
	if err != nil {
		t.Fatal(err)
	}
	secondContents, err := os.ReadFile(second)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(firstContents, secondContents) {
		t.Fatal("identical source content produced different archives")
	}

	gzipReader, err := gzip.NewReader(bytes.NewReader(firstContents))
	if err != nil {
		t.Fatal(err)
	}
	defer gzipReader.Close()
	tarReader := tar.NewReader(gzipReader)
	var files []string
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		files = append(files, header.Name)
		if !header.ModTime.Equal(epoch) {
			t.Fatalf("%s has non-deterministic mtime %s", header.Name, header.ModTime)
		}
	}
	want := []string{"completions/", "completions/lana.bash", "lana"}
	if !equalStrings(files, want) {
		t.Fatalf("archive files = %v, want %v", files, want)
	}
}

func equalStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}
