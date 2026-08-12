package output

import (
	"bytes"
	"strings"
	"testing"
)

func TestWriterWithColors(t *testing.T) {
	var buf bytes.Buffer
	w := NewWriter(&buf, true)

	w.Println(Green, "success: %s", "test")
	output := buf.String()
	if !strings.Contains(output, Green) {
		t.Fatal("expected green color code in output")
	}
	if !strings.Contains(output, "success: test") {
		t.Fatalf("expected success message, got: %s", output)
	}
}

func TestWriterWithoutColors(t *testing.T) {
	var buf bytes.Buffer
	w := NewWriter(&buf, false)

	w.Println(Green, "success: %s", "test")
	output := buf.String()
	if strings.Contains(output, "\033") {
		t.Fatal("expected no color codes in output")
	}
	if !strings.Contains(output, "success: test") {
		t.Fatalf("expected success message, got: %s", output)
	}
}

func TestProgressBar(t *testing.T) {
	var buf bytes.Buffer
	w := NewWriter(&buf, false)
	pb := NewProgressBar(w, 10, "Processing")

	pb.Update(5)
	output := buf.String()
	if !strings.Contains(output, "50%") {
		t.Fatalf("expected 50%% in output, got: %s", output)
	}

	pb.Finish()
	output = buf.String()
	if !strings.Contains(output, "100%") {
		t.Fatalf("expected 100%% in output, got: %s", output)
	}
}

func TestProgressBarClamp(t *testing.T) {
	var buf bytes.Buffer
	w := NewWriter(&buf, false)
	pb := NewProgressBar(w, 10, "Test")

	pb.Update(-5)
	output := buf.String()
	if !strings.Contains(output, "0%") {
		t.Fatalf("expected 0%% for negative value, got: %s", output)
	}

	pb.Update(15)
	output = buf.String()
	if !strings.Contains(output, "100%") {
		t.Fatalf("expected 100%% for value > total, got: %s", output)
	}
}

func TestSpinner(t *testing.T) {
	var buf bytes.Buffer
	w := NewWriter(&buf, false)
	spinner := NewSpinner(w, "Loading")

	spinner.Start()
	// Give it a moment to animate
	spinner.Stop()

	output := buf.String()
	// Spinner may or may not have written output depending on timing
	// Just verify it doesn't panic
	_ = output
}

func TestSpinnerSuccess(t *testing.T) {
	var buf bytes.Buffer
	w := NewWriter(&buf, false)
	spinner := NewSpinner(w, "Done")

	spinner.Start()
	spinner.Success()

	output := buf.String()
	if !strings.Contains(output, "Done") {
		t.Fatalf("expected spinner label in output, got: %s", output)
	}
}

func TestSpinnerFail(t *testing.T) {
	var buf bytes.Buffer
	w := NewWriter(&buf, false)
	spinner := NewSpinner(w, "Failed")

	spinner.Start()
	spinner.Fail()

	output := buf.String()
	if !strings.Contains(output, "Failed") {
		t.Fatalf("expected spinner label in output, got: %s", output)
	}
}

func TestIsTerminal(t *testing.T) {
	// bytes.Buffer is not a terminal
	var buf bytes.Buffer
	if isTerminal(&buf) {
		t.Fatal("bytes.Buffer should not be a terminal")
	}
}

func TestSuccessMessage(t *testing.T) {
	var buf bytes.Buffer
	w := NewWriter(&buf, false)

	w.Success("operation completed")
	output := buf.String()
	if !strings.Contains(output, "✓ operation completed") {
		t.Fatalf("expected success message, got: %s", output)
	}
}

func TestErrorMessage(t *testing.T) {
	var buf bytes.Buffer
	w := NewWriter(&buf, false)

	w.Error("something failed")
	output := buf.String()
	if !strings.Contains(output, "✗ something failed") {
		t.Fatalf("expected error message, got: %s", output)
	}
}

func TestWarnMessage(t *testing.T) {
	var buf bytes.Buffer
	w := NewWriter(&buf, false)

	w.Warn("be careful")
	output := buf.String()
	if !strings.Contains(output, "⚠ be careful") {
		t.Fatalf("expected warn message, got: %s", output)
	}
}

func TestInfoMessage(t *testing.T) {
	var buf bytes.Buffer
	w := NewWriter(&buf, false)

	w.Info("just info")
	output := buf.String()
	if !strings.Contains(output, "ℹ just info") {
		t.Fatalf("expected info message, got: %s", output)
	}
}
