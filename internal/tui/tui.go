package tui

import (
	"os"
	"strings"
)

var ColorsEnabled = os.Getenv("NO_COLOR") == ""

func StripANSI(s string) string {
	if !strings.Contains(s, "\x1b") {
		return s
	}
	var buf strings.Builder
	buf.Grow(len(s))
	inEscape := false
	for _, r := range s {
		if inEscape {
			if (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') {
				inEscape = false
			}
			continue
		}
		if r == '\x1b' {
			inEscape = true
			continue
		}
		buf.WriteRune(r)
	}
	return buf.String()
}
