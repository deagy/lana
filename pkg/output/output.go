// Package output provides rich terminal output with colors and formatting.
package output

import (
	"fmt"
	"io"
	"os"
	"strings"
)

// Color codes for terminal output.
const (
	Reset     = "\033[0m"
	Red       = "\033[31m"
	Green     = "\033[32m"
	Yellow    = "\033[33m"
	Blue      = "\033[34m"
	Purple    = "\033[35m"
	Cyan      = "\033[36m"
	White     = "\033[37m"
	Bold      = "\033[1m"
	Dim       = "\033[2m"
	Underline = "\033[4m"
)

// Writer provides formatted output with optional colors.
type Writer struct {
	out          io.Writer
	enableColors bool
}

// NewWriter creates a new output writer.
func NewWriter(out io.Writer, enableColors bool) *Writer {
	return &Writer{
		out:          out,
		enableColors: enableColors,
	}
}

// NewDefaultWriter creates a writer that auto-detects color support.
func NewDefaultWriter(out io.Writer) *Writer {
	return NewWriter(out, isTerminal(out))
}

// isTerminal checks if the writer is a terminal.
func isTerminal(w io.Writer) bool {
	if f, ok := w.(*os.File); ok {
		stat, err := f.Stat()
		if err != nil {
			return false
		}
		return (stat.Mode() & os.ModeCharDevice) != 0
	}
	return false
}

// Color wraps text with ANSI color codes.
func (w *Writer) Color(color, text string) string {
	if !w.enableColors {
		return text
	}
	return color + text + Reset
}

// Red wraps text in red.
func (w *Writer) Red(text string) string {
	return w.Color(Red, text)
}

// Green wraps text in green.
func (w *Writer) Green(text string) string {
	return w.Color(Green, text)
}

// Yellow wraps text in yellow.
func (w *Writer) Yellow(text string) string {
	return w.Color(Yellow, text)
}

// Blue wraps text in blue.
func (w *Writer) Blue(text string) string {
	return w.Color(Blue, text)
}

// Bold wraps text in bold.
func (w *Writer) Bold(text string) string {
	return w.Color(Bold, text)
}

// Print outputs text with optional color.
func (w *Writer) Print(color, format string, a ...interface{}) {
	text := fmt.Sprintf(format, a...)
	if color != "" {
		text = w.Color(color, text)
	}
	fmt.Fprint(w.out, text)
}

// Println outputs colored text with a newline.
func (w *Writer) Println(color, format string, a ...interface{}) {
	w.Print(color, format, a...)
	fmt.Fprintln(w.out)
}

// Success outputs a success message in green.
func (w *Writer) Success(format string, a ...interface{}) {
	w.Println(Green, "✓ "+format, a...)
}

// Error outputs an error message in red.
func (w *Writer) Error(format string, a ...interface{}) {
	w.Println(Red, "✗ "+format, a...)
}

// Warn outputs a warning message in yellow.
func (w *Writer) Warn(format string, a ...interface{}) {
	w.Println(Yellow, "⚠ "+format, a...)
}

// Info outputs an info message in blue.
func (w *Writer) Info(format string, a ...interface{}) {
	w.Println(Blue, "ℹ "+format, a...)
}

// Header outputs a bold header.
func (w *Writer) Header(format string, a ...interface{}) {
	w.Println(Bold, format, a...)
}

// ProgressBar represents a progress bar.
type ProgressBar struct {
	writer  *Writer
	total   int
	current int
	label   string
	width   int
}

// NewProgressBar creates a new progress bar.
func NewProgressBar(writer *Writer, total int, label string) *ProgressBar {
	if total <= 0 {
		total = 100
	}
	return &ProgressBar{
		writer: writer,
		total:  total,
		label:  label,
		width:  40,
	}
}

// Update updates the progress bar.
func (p *ProgressBar) Update(current int) {
	p.current = current
	p.render()
}

// Increment increments the progress by 1.
func (p *ProgressBar) Increment() {
	p.current++
	p.render()
}

// Finish completes the progress bar.
func (p *ProgressBar) Finish() {
	p.current = p.total
	p.render()
	fmt.Fprintln(p.writer.out)
}

func (p *ProgressBar) render() {
	if p.current < 0 {
		p.current = 0
	}
	if p.current > p.total {
		p.current = p.total
	}

	percent := float64(p.current) / float64(p.total)
	filled := int(percent * float64(p.width))

	bar := strings.Repeat("█", filled) + strings.Repeat("░", p.width-filled)
	status := p.writer.Green("✓")
	if percent < 1.0 {
		status = p.writer.Blue("⟳")
	}

	fmt.Fprintf(p.writer.out, "\r%s %s [%s] %3d%% (%d/%d)",
		p.writer.Bold(p.label),
		status,
		bar,
		int(percent*100),
		p.current,
		p.total,
	)
}

// Spinner represents a loading spinner.
type Spinner struct {
	writer  *Writer
	label   string
	running bool
	frames  []string
	current int
}

// NewSpinner creates a new spinner.
func NewSpinner(writer *Writer, label string) *Spinner {
	return &Spinner{
		writer: writer,
		label:  label,
		frames: []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"},
	}
}

// Start starts the spinner.
func (s *Spinner) Start() {
	s.running = true
	go s.animate()
}

// Stop stops the spinner.
func (s *Spinner) Stop() {
	s.running = false
	fmt.Fprintln(s.writer.out)
}

// Success marks the spinner as successful.
func (s *Spinner) Success() {
	s.Stop()
	s.writer.Success("%s", s.label)
}

// Fail marks the spinner as failed.
func (s *Spinner) Fail() {
	s.Stop()
	s.writer.Error("%s", s.label)
}

func (s *Spinner) animate() {
	i := 0
	for s.running {
		frame := s.frames[i%len(s.frames)]
		fmt.Fprintf(s.writer.out, "\r%s %s", frame, s.writer.Bold(s.label))
		i++
		// Simple delay
		for j := 0; j < 10000000; j++ {
			// Busy wait
		}
	}
}
