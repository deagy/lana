package logger

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInitCreatesPrivateLogDirectoryAndFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "logs", "lana.log")
	lg := Init(Options{ToFile: true, FilePath: path})
	lg.Info("private log")

	assertPermissions(t, filepath.Dir(path), 0700)
	assertPermissions(t, path, 0600)

	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read log file: %v", err)
	}
	if !strings.Contains(string(contents), "private log") {
		t.Fatalf("log file does not contain the emitted record: %q", contents)
	}
}

func TestInitTightensConfiguredExistingLogPathOnly(t *testing.T) {
	base := t.TempDir()
	parent := filepath.Join(base, "parent")
	dir := filepath.Join(parent, "logs")
	path := filepath.Join(dir, "lana.log")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("create log directory: %v", err)
	}
	if err := os.WriteFile(path, []byte("existing\n"), 0644); err != nil {
		t.Fatalf("create log file: %v", err)
	}
	if err := os.Chmod(parent, 0755); err != nil {
		t.Fatalf("set parent permissions: %v", err)
	}

	Init(Options{ToFile: true, FilePath: path})

	assertPermissions(t, dir, 0700)
	assertPermissions(t, path, 0600)
	assertPermissions(t, parent, 0755)
}

func TestInitWithBareFilenameDoesNotChangeCurrentDirectoryPermissions(t *testing.T) {
	dir := t.TempDir()
	if err := os.Chmod(dir, 0755); err != nil {
		t.Fatalf("set current directory permissions: %v", err)
	}
	t.Chdir(dir)

	Init(Options{ToFile: true, FilePath: "lana.log"})

	assertPermissions(t, dir, 0755)
	assertPermissions(t, filepath.Join(dir, "lana.log"), 0600)
}

func TestInitDoesNotFollowSymlinkedLogPaths(t *testing.T) {
	t.Run("directory", func(t *testing.T) {
		base := t.TempDir()
		externalDir := filepath.Join(base, "external")
		if err := os.Mkdir(externalDir, 0755); err != nil {
			t.Fatalf("create external directory: %v", err)
		}
		link := filepath.Join(base, "log-link")
		if err := os.Symlink(externalDir, link); err != nil {
			t.Skipf("symlinks unavailable: %v", err)
		}

		Init(Options{ToFile: true, FilePath: filepath.Join(link, "lana.log")})

		assertPermissions(t, externalDir, 0755)
		if _, err := os.Stat(filepath.Join(externalDir, "lana.log")); !os.IsNotExist(err) {
			t.Fatalf("symlink target was used as a log directory: %v", err)
		}
	})

	t.Run("file", func(t *testing.T) {
		base := t.TempDir()
		dir := filepath.Join(base, "logs")
		if err := os.Mkdir(dir, 0700); err != nil {
			t.Fatalf("create log directory: %v", err)
		}
		externalFile := filepath.Join(base, "external.log")
		if err := os.WriteFile(externalFile, []byte("external\n"), 0644); err != nil {
			t.Fatalf("create external file: %v", err)
		}
		path := filepath.Join(dir, "lana.log")
		if err := os.Symlink(externalFile, path); err != nil {
			t.Skipf("symlinks unavailable: %v", err)
		}

		Init(Options{ToFile: true, FilePath: path})

		assertPermissions(t, externalFile, 0644)
		contents, err := os.ReadFile(externalFile)
		if err != nil {
			t.Fatalf("read external file: %v", err)
		}
		if string(contents) != "external\n" {
			t.Fatalf("external file was used as a log file: %q", contents)
		}
	})
}

func assertPermissions(t *testing.T, path string, want os.FileMode) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat %s: %v", path, err)
	}
	if got := info.Mode().Perm(); got != want {
		t.Errorf("permissions for %s = %04o, want %04o", path, got, want)
	}
}
