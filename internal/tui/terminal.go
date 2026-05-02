package tui

import (
	"os"
	"strconv"

	"golang.org/x/term"
)

const (
	DefaultRows = 24
	DefaultCols = 80
)

func TerminalSize() (rows, cols int) {
	if r := os.Getenv("TRY_HEIGHT"); r != "" {
		if n, err := strconv.Atoi(r); err == nil && n > 0 {
			rows = n
		}
	}
	if c := os.Getenv("TRY_WIDTH"); c != "" {
		if n, err := strconv.Atoi(c); err == nil && n > 0 {
			cols = n
		}
	}

	streams := []int{
		int(os.Stderr.Fd()),
		int(os.Stdout.Fd()),
		int(os.Stdin.Fd()),
	}

	for _, fd := range streams {
		if rows > 0 && cols > 0 {
			break
		}
		if w, h, err := term.GetSize(fd); err == nil {
			if rows == 0 {
				rows = h
			}
			if cols == 0 {
				cols = w
			}
		}
	}

	if rows <= 0 {
		rows = DefaultRows
	}
	if cols <= 0 {
		cols = DefaultCols
	}
	return
}
