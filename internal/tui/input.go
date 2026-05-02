package tui

import (
	"strings"
)

type InputField struct {
	Text        string
	Cursor      int
	Placeholder string
}

func NewInputField(placeholder, text string, cursor int) *InputField {
	if cursor < 0 {
		cursor = 0
	}
	if cursor > len(text) {
		cursor = len(text)
	}
	return &InputField{
		Placeholder: placeholder,
		Text:        text,
		Cursor:      cursor,
	}
}

func (f *InputField) String() string {
	if f.Text == "" {
		if ColorsEnabled {
			return DimText(f.Placeholder)
		}
		return f.Placeholder
	}

	var buf strings.Builder
	before := f.Text[:f.Cursor]
	cursorChar := " "
	if f.Cursor < len(f.Text) {
		cursorChar = string(f.Text[f.Cursor])
	}

	after := ""
	if f.Cursor < len(f.Text) {
		after = f.Text[f.Cursor+1:]
	}

	buf.WriteString(before)
	if ColorsEnabled {
		buf.WriteString(InputCursorOn)
	}
	buf.WriteString(cursorChar)
	if ColorsEnabled {
		buf.WriteString(InputCursorOff)
	}
	buf.WriteString(after)

	return buf.String()
}

func (f *InputField) CursorPosition() int {
	return f.Cursor
}
