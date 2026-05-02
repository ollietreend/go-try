package tui

import (
	"strings"
)

type segmentType int

const (
	segText segmentType = iota
	segFill
	segEmoji
)

type segment struct {
	typ   segmentType
	text  string
	style string
	emojiWidth int
}

type Writer struct {
	segments    []segment
	zIndex      int
	hasWide     bool
	widthDelta  int
}

func NewWriter(zIndex int) *Writer {
	return &Writer{
		zIndex: zIndex,
	}
}

func (w *Writer) Write(s string) *Writer {
	if s == "" {
		return w
	}
	w.segments = append(w.segments, segment{typ: segText, text: s})
	return w
}

func (w *Writer) WriteDim(s string) *Writer {
	if s == "" {
		return w
	}
	if ColorsEnabled {
		w.segments = append(w.segments, segment{typ: segText, text: DimText(s)})
	} else {
		w.segments = append(w.segments, segment{typ: segText, text: s})
	}
	return w
}

func (w *Writer) WriteBold(s string) *Writer {
	if s == "" {
		return w
	}
	if ColorsEnabled {
		w.segments = append(w.segments, segment{typ: segText, text: BoldText(s)})
	} else {
		w.segments = append(w.segments, segment{typ: segText, text: s})
	}
	return w
}

func (w *Writer) WriteHighlight(s string) *Writer {
	if s == "" {
		return w
	}
	if ColorsEnabled {
		w.segments = append(w.segments, segment{typ: segText, text: HighlightText(s)})
	} else {
		w.segments = append(w.segments, segment{typ: segText, text: s})
	}
	return w
}

func (w *Writer) WriteAccent(s string) *Writer {
	if s == "" {
		return w
	}
	if ColorsEnabled {
		w.segments = append(w.segments, segment{typ: segText, text: AccentText(s)})
	} else {
		w.segments = append(w.segments, segment{typ: segText, text: s})
	}
	return w
}

func (w *Writer) WriteFill(char string) *Writer {
	if char == "" {
		char = " "
	}
	w.segments = append(w.segments, segment{typ: segFill, text: char})
	return w
}

func (w *Writer) WriteEmoji(char string) *Writer {
	width := 0
	for _, r := range char {
		cw := CharWidth(r)
		if cw > 0 {
			width += cw
		}
	}
	delta := width - len(char)
	w.segments = append(w.segments, segment{
		typ:        segEmoji,
		text:       char,
		emojiWidth: width,
	})
	if delta > 0 {
		w.hasWide = true
		w.widthDelta += delta
	}
	return w
}

func (w *Writer) HasWide() bool {
	return w.hasWide
}

func (w *Writer) IsEmpty() bool {
	return len(w.segments) == 0
}

func (w *Writer) String() string {
	return w.StringWidth(0)
}

func (w *Writer) StringWidth(width int) string {
	var buf strings.Builder
	for _, seg := range w.segments {
		switch seg.typ {
		case segFill:
			if width > 0 {
				rendered := buf.String()
				used := VisibleWidth(rendered)
				remaining := (width - 1) - used
				if remaining > 0 {
					pattern := seg.text
					if pattern == "" {
						pattern = " "
					}
					buf.WriteString(fillPattern(pattern, remaining))
				}
			}
		case segEmoji:
			buf.WriteString(seg.text)
		default:
			buf.WriteString(seg.text)
		}
	}
	return buf.String()
}

func (w *Writer) VisibleWidth() int {
	raw := w.String()
	return VisibleWidth(raw)
}

func fillPattern(pattern string, width int) string {
	if width <= 0 {
		return ""
	}
	count := width / len(pattern)
	remainder := width % len(pattern)
	return strings.Repeat(pattern, count) + pattern[:remainder]
}
