package tui

import "fmt"

const (
	ClearEOL       = "\x1b[K"
	ClearEOS       = "\x1b[J"
	ClearScreen    = "\x1b[2J"
	Home           = "\x1b[H"
	HideCursor     = "\x1b[?25l"
	ShowCursor     = "\x1b[?25h"
	CursorBlink    = "\x1b[1 q"
	CursorSteady   = "\x1b[2 q"
	CursorDefault  = "\x1b[0 q"
	AltScreenOn    = "\x1b[?1049h"
	AltScreenOff   = "\x1b[?1049l"
	Reset          = "\x1b[0m"
	ResetFG        = "\x1b[39m"
	ResetBG        = "\x1b[49m"
	ResetIntensity = "\x1b[22m"
	Bold           = "\x1b[1m"
	Dim            = "\x1b[2m"
)

func Fg(code int) string    { return fmt.Sprintf("\x1b[38;5;%dm", code) }
func Bg(code int) string    { return fmt.Sprintf("\x1b[48;5;%dm", code) }
func MoveCol(col int) string { return fmt.Sprintf("\x1b[%dG", col) }
func SetTitle(t string) string { return fmt.Sprintf("\x1b]2;%s\a", t) }
