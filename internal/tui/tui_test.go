package tui

import (
	"bytes"
	"strings"
	"testing"
)

func TestScreenRender(t *testing.T) {
	var buf bytes.Buffer
	s := NewScreen(&buf, 80, 24)

	s.Header.AddLine("").Write().WriteAccent(" Try Directory Selection").WriteEmoji("🏠")
	s.Header.AddLine("").Write().WriteDim("───────────────────────────────────────────────────────────────────────────────")

	searchLine := s.Header.AddLine("")
	searchLine.Write().WriteDim("Search: ")
	searchLine.Write().Write("redis")
	searchLine.MarkHasInput(8)

	s.Header.AddLine("").Write().WriteDim("───────────────────────────────────────────────────────────────────────────────")

	bodyLine := s.Body.AddLine("")
	bodyLine.Write().Write("→ 📁 2025-08-17-redis-experiment")
	bodyLine.Right().WriteDim("2h, 18.5")

	s.Footer.AddLine("").Write().WriteDim("───────────────────────────────────────────────────────────────────────────────")
	s.Footer.AddLine("").Write().WriteDim("↑/↓: Navigate  Enter: Select  ^R: Rename  ^D: Delete  Esc: Cancel")

	s.SetInput("", "redis", 5)

	s.Flush()

	out := buf.String()

	if !strings.Contains(out, "redis") {
		t.Error("output should contain 'redis'")
	}
	if !strings.Contains(out, Home) {
		t.Error("output should start with HOME cursor sequence")
	}
	if !strings.Contains(out, Reset) {
		t.Error("output should contain reset sequence")
	}
}

func TestPainText(t *testing.T) {
	s := "hello"
	wrapped := BoldText(s)
	if !ColorsEnabled {
		if wrapped != "hello" {
			t.Error("should return plain text when colors disabled")
		}
	} else {
		if !strings.Contains(wrapped, Bold) {
			t.Error("should contain bold ANSI code")
		}
	}
}

func TestVisibleWidth(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		{"hello", 5},
		{"🏠", 2},
		{"📁", 2},
		{"hello🏠", 7},
	}
	for _, tt := range tests {
		got := VisibleWidth(tt.input)
		if got != tt.want {
			t.Errorf("VisibleWidth(%q) = %d, want %d", tt.input, got, tt.want)
		}
	}
}

func TestTruncate(t *testing.T) {
	result := Truncate("hello world", 5, "…")
	if VisibleWidth(result) > 5 {
		t.Errorf("truncated string too long: %q", result)
	}
}

func TestInputField(t *testing.T) {
	f := NewInputField("placeholder", "test", 2)
	out := f.String()
	if !strings.Contains(out, "te") {
		t.Errorf("input should contain text before cursor, got: %q", out)
	}
}

func TestWriterFill(t *testing.T) {
	w := NewWriter(0)
	w.Write("hello")
	w.WriteFill("─")
	result := w.StringWidth(20)
	if !strings.Contains(result, "hello") {
		t.Errorf("fill result should contain text: %q", result)
	}
}
