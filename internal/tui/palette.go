package tui

var (
	Header    = "\x1b[1;38;5;114m"
	Accent    = "\x1b[1;38;5;214m"
	Highlight = "\x1b[1;33m"
	Muted     = Fg(245)
	Match     = "\x1b[1;38;5;226m"
	InputHint = Fg(244)

	InputCursorOn  = "\x1b[7m"
	InputCursorOff = "\x1b[27m"

	SelectedBG = Bg(238)
	DangerBG   = Bg(52)
)
