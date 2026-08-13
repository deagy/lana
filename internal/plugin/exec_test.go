package plugin

import (
	"bytes"
	"context"
	"path/filepath"
	"testing"
)

func TestRun(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		stdin   string
		wantOut string
		wantErr bool
	}{
		{
			name:    "simple args",
			args:    []string{"hello", "world"},
			wantOut: "arg: hello\narg: world\n",
		},
		{
			name:    "no args",
			args:    []string{},
			wantOut: "",
		},
		{
			name:    "single arg with spaces",
			args:    []string{"hello world"},
			wantOut: "arg: hello world\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Find fixture
			fixtureDir, err := filepath.Abs(filepath.Join("testdata", "fixture"))
			if err != nil {
				t.Fatalf("abs failed: %v", err)
			}

			manifest := &Manifest{
				Name:       "echoplugin",
				Entrypoint: "echoplugin.sh",
			}

			var stdout, stderr bytes.Buffer
			var stdin bytes.Buffer
			stdin.WriteString(tt.stdin)

			ctx := context.Background()
			err = Run(ctx, fixtureDir, manifest, tt.args, &stdin, &stdout, &stderr)

			if (err != nil) != tt.wantErr {
				t.Errorf("Run() error = %v, wantErr %v", err, tt.wantErr)
			}

			if stdout.String() != tt.wantOut {
				t.Errorf("Run() stdout = %q, want %q", stdout.String(), tt.wantOut)
			}
		})
	}
}

func TestRunContextCancellation(t *testing.T) {
	fixtureDir, err := filepath.Abs(filepath.Join("testdata", "fixture"))
	if err != nil {
		t.Fatalf("abs failed: %v", err)
	}

	manifest := &Manifest{
		Name:       "echoplugin",
		Entrypoint: "echoplugin.sh",
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	var stdout, stderr bytes.Buffer
	err = Run(ctx, fixtureDir, manifest, []string{}, nil, &stdout, &stderr)

	// Should get a context canceled error
	if err == nil {
		t.Errorf("Run() error = nil, want context canceled error")
	}
}

func TestRunWrongDirectory(t *testing.T) {
	// Try to run a plugin that doesn't exist
	fixtureDir := "/nonexistent/plugin"

	manifest := &Manifest{
		Name:       "missing",
		Entrypoint: "run.sh",
	}

	var stdout, stderr bytes.Buffer
	ctx := context.Background()
	err := Run(ctx, fixtureDir, manifest, []string{}, nil, &stdout, &stderr)

	if err == nil {
		t.Errorf("Run() error = nil, want exec error")
	}
}
