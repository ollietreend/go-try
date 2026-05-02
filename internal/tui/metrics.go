package tui

import (
	"strings"
	"unicode/utf8"
)

func CharWidth(r rune) int {
	if r >= 0xFE00 && r <= 0xFE0F {
		return 0
	}
	if r >= 0x1F300 && r <= 0x1FAFF {
		return 2
	}
	return 1
}

func VisibleWidth(s string) int {
	if !strings.Contains(s, "\x1b") {
		width := 0
		for _, r := range s {
			width += CharWidth(r)
		}
		return width
	}

	noANSI := StripANSI(s)
	width := 0
	for _, r := range noANSI {
		width += CharWidth(r)
	}
	return width
}

func Truncate(s string, maxWidth int, overflow string) string {
	visWidth := VisibleWidth(s)
	if visWidth <= maxWidth {
		return s
	}

	overflowWidth := VisibleWidth(overflow)
	target := maxWidth - overflowWidth
	if target < 0 {
		target = 0
	}

	var result strings.Builder
	width := 0
	inEscape := false

	for _, r := range s {
		if inEscape {
			result.WriteRune(r)
			if (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') {
				inEscape = false
			}
			continue
		}

		if r == '\x1b' {
			inEscape = true
			result.WriteRune(r)
			continue
		}

		cw := CharWidth(r)
		if width+cw > target {
			break
		}
		result.WriteRune(r)
		width += cw
	}

	return strings.TrimRight(result.String(), " ") + overflow
}

func TruncateFromStart(s string, maxWidth int) string {
	visWidth := VisibleWidth(s)
	if visWidth <= maxWidth {
		return s
	}

	var leadingEscapes strings.Builder
	inEscape := false
	bodyStart := 0

	for i, r := range s {
		if inEscape {
			leadingEscapes.WriteRune(r)
			if (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') {
				inEscape = false
			}
		} else if r == '\x1b' {
			inEscape = true
			leadingEscapes.WriteRune(r)
		} else {
			bodyStart = i
			break
		}
	}

	charsToSkip := visWidth - maxWidth
	skipped := 0
	var result strings.Builder
	inEscape = false

	for i, r := range s {
		if i < bodyStart {
			continue
		}
		if inEscape {
			if skipped >= charsToSkip {
				result.WriteRune(r)
			}
			if (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') {
				inEscape = false
			}
			continue
		}
		if r == '\x1b' {
			inEscape = true
			if skipped >= charsToSkip {
				result.WriteRune(r)
			}
			continue
		}
		cw := CharWidth(r)
		if skipped < charsToSkip {
			skipped += cw
		} else {
			result.WriteRune(r)
		}
	}

	return leadingEscapes.String() + result.String()
}

func TruncateVisible(s string, maxWidth int) string {
	visWidth := VisibleWidth(s)
	if visWidth <= maxWidth {
		return s
	}

	overflowRune, _ := utf8.DecodeRuneInString("…")
	return Truncate(s, maxWidth-1, string(overflowRune))
}
