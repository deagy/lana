package scripts_test

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

const nonGitRegressionEnvironment = "LANA_NONGIT_RELEASE_REGRESSION"

func TestMakeCIWithoutGitHeadUsesDeterministicMetadata(t *testing.T) {
	if os.Getenv(nonGitRegressionEnvironment) != "" {
		t.Skip("avoid recursively running make ci from its own regression test")
	}

	repoRoot := repositoryRoot(t)
	copyRoot := filepath.Join(t.TempDir(), "lana")
	for _, name := range []string{"cmd", "internal", "pkg", "scripts", "testdata", "go.mod", "go.sum", "Makefile"} {
		copyPath(t, filepath.Join(repoRoot, name), filepath.Join(copyRoot, name))
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	command := exec.CommandContext(ctx, "make", "ci")
	command.Dir = copyRoot
	command.Env = regressionEnvironment(copyRoot)
	output, err := command.CombinedOutput()
	if ctx.Err() != nil {
		t.Fatalf("make ci did not complete: %v\n%s", ctx.Err(), output)
	}
	if err != nil {
		t.Fatalf("make ci without a Git HEAD failed: %v\n%s", err, output)
	}

	archive := filepath.Join(copyRoot, "dist", fmt.Sprintf("lana_dev_%s_%s.tar.gz", runtime.GOOS, runtime.GOARCH))
	binary := extractArchiveFile(t, archive, "lana")
	versionOutput, err := exec.Command(binary, "--version").CombinedOutput()
	if err != nil {
		t.Fatalf("run packaged binary: %v\n%s", err, versionOutput)
	}
	if got, want := string(versionOutput), "lana version dev (commit: unknown, built: 1970-01-01T00:00:00Z,"; !strings.Contains(got, want) {
		t.Fatalf("packaged version metadata = %q, want it to contain %q", got, want)
	}
}

func repositoryRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate test file")
	}
	return filepath.Dir(filepath.Dir(file))
}

func regressionEnvironment(copyRoot string) []string {
	environment := make([]string, 0, len(os.Environ())+2)
	for _, entry := range os.Environ() {
		if strings.HasPrefix(entry, "SOURCE_DATE_EPOCH=") || strings.HasPrefix(entry, "VERSION=") || strings.HasPrefix(entry, "COMMIT=") || strings.HasPrefix(entry, "DIST_DIR=") {
			continue
		}
		environment = append(environment, entry)
	}
	return append(environment,
		nonGitRegressionEnvironment+"=1",
		"GIT_CEILING_DIRECTORIES="+copyRoot,
	)
}

func copyPath(t *testing.T, source, destination string) {
	t.Helper()
	info, err := os.Lstat(source)
	if err != nil {
		t.Fatal(err)
	}
	if !info.IsDir() {
		copyFile(t, source, destination, info.Mode())
		return
	}
	if err := filepath.WalkDir(source, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relative, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		target := filepath.Join(destination, relative)
		if entry.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		copyFile(t, path, target, info.Mode())
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func copyFile(t *testing.T, source, destination string, mode os.FileMode) {
	t.Helper()
	input, err := os.Open(source)
	if err != nil {
		t.Fatal(err)
	}
	defer input.Close()
	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		t.Fatal(err)
	}
	output, err := os.OpenFile(destination, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode.Perm())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.Copy(output, input); err != nil {
		output.Close()
		t.Fatal(err)
	}
	if err := output.Close(); err != nil {
		t.Fatal(err)
	}
}

func extractArchiveFile(t *testing.T, archive, name string) string {
	t.Helper()
	input, err := os.Open(archive)
	if err != nil {
		t.Fatal(err)
	}
	defer input.Close()
	gzipReader, err := gzip.NewReader(input)
	if err != nil {
		t.Fatal(err)
	}
	defer gzipReader.Close()
	tarReader := tar.NewReader(gzipReader)
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		if header.Name != name {
			continue
		}
		path := filepath.Join(t.TempDir(), filepath.Base(name))
		output, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, os.FileMode(header.Mode))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := io.Copy(output, tarReader); err != nil {
			output.Close()
			t.Fatal(err)
		}
		if err := output.Close(); err != nil {
			t.Fatal(err)
		}
		return path
	}
	t.Fatalf("%s does not contain %s", archive, name)
	return ""
}
