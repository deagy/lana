package knowledge

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCommandIngestSearchListAndSourceDeletion(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "fixture.md")
	if err := os.WriteFile(source, []byte("local retrieval with citations"), 0o600); err != nil {
		t.Fatal(err)
	}
	store := filepath.Join(root, "store")

	ingested := run(t, "--store", store, "ingest", "--source", "fixture", "--tag", "docs", "--json", source)
	var ingest struct {
		Added  int    `json:"added"`
		Source string `json:"source"`
	}
	if err := json.Unmarshal([]byte(ingested), &ingest); err != nil || ingest.Added != 1 || ingest.Source != "fixture" {
		t.Fatalf("ingest output = %q, %v", ingested, err)
	}

	searched := run(t, "--store", store, "search", "--source", "fixture", "--tag", "DOCS", "--json", "retrieval")
	if !strings.Contains(searched, `"citation"`) || !strings.Contains(searched, `"content_hash"`) || !strings.Contains(searched, `"document_id"`) {
		t.Fatalf("search does not expose citations: %s", searched)
	}
	listed := run(t, "--store", store, "list", "--source", "fixture")
	if !strings.Contains(listed, "source=fixture") {
		t.Fatalf("list = %q", listed)
	}
	removed := run(t, "--store", store, "remove", "--source", "fixture", "--force")
	if !strings.Contains(removed, "1 documents") {
		t.Fatalf("remove = %q", removed)
	}
}

func TestCommandRequiresForceForDeletion(t *testing.T) {
	cmd := NewCommand()
	cmd.SetArgs([]string{"remove", "doc-example"})
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "--force") {
		t.Fatalf("remove error = %v", err)
	}
}

func TestHumanOutputEscapesTerminalControlsButJSONPreservesData(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "fixture.txt")
	malicious := "needle \x1b]52;c;clipboard\a\nnext"
	if err := os.WriteFile(source, []byte(malicious), 0o600); err != nil {
		t.Fatal(err)
	}
	store := filepath.Join(root, "store")
	run(t, "--store", store, "ingest", "--source", "fixture", source)
	human := run(t, "--store", store, "search", "needle")
	if strings.ContainsRune(human, '\x1b') || strings.ContainsRune(human, '\a') {
		t.Fatalf("human output includes terminal control: %q", human)
	}
	if !strings.Contains(human, `\x1B]52;c;clipboard\x07\x0Anext`) {
		t.Fatalf("human output did not visibly escape content: %q", human)
	}
	machine := run(t, "--store", store, "search", "--json", "needle")
	if !strings.Contains(machine, `\u001b`) {
		t.Fatalf("JSON output did not preserve escaped control data: %q", machine)
	}
}

func run(t *testing.T, args ...string) string {
	t.Helper()
	cmd := NewCommand()
	var out, errOut bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	cmd.SetArgs(args)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("knowledge %v: %v (%s)", args, err, errOut.String())
	}
	return out.String()
}
