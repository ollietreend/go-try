package tui

import (
	"fmt"
	"io"
	"strings"
)

type Section struct {
	Lines []*Line
}

func NewSection() *Section {
	return &Section{}
}

func (s *Section) AddLine(background string) *Line {
	l := &Line{
		left:   NewWriter(1),
		center: nil,
		right:  nil,
		bg:     background,
	}
	s.Lines = append(s.Lines, l)
	return l
}

func (s *Section) Clear() {
	s.Lines = s.Lines[:0]
}

type Line struct {
	left       *Writer
	center     *Writer
	right      *Writer
	bg         string
	hasInput   bool
	inputPrefixWidth int
}

func (l *Line) Write() *Writer {
	return l.left
}

func (l *Line) Center() *Writer {
	if l.center == nil {
		l.center = NewWriter(2)
	}
	return l.center
}

func (l *Line) Right() *Writer {
	if l.right == nil {
		l.right = NewWriter(0)
	}
	return l.right
}

func (l *Line) HasInput() bool {
	return l.hasInput
}

func (l *Line) MarkHasInput(prefixWidth int) {
	l.hasInput = true
	l.inputPrefixWidth = prefixWidth
}

func (l *Line) CursorColumn(input *InputField, width int) int {
	return l.inputPrefixWidth + input.Cursor + 1
}

func (l *Line) render(buf *strings.Builder, width int, trailingNewline bool) {
	buf.WriteString("\r")
	buf.WriteString(ClearEOL)

	if l.bg != "" && ColorsEnabled {
		buf.WriteString(l.bg)
	}

	maxContent := width - 1
	if maxContent < 1 {
		maxContent = 1
	}

	leftText := l.left.StringWidth(width)
	centerText := ""
	if l.center != nil {
		centerText = l.center.StringWidth(width)
	}
	rightText := ""
	if l.right != nil {
		rightText = l.right.StringWidth(width)
	}

	leftText = TruncateVisible(leftText, maxContent)
	leftWidth := VisibleWidth(leftText)

	if centerText != "" {
		maxCenter := maxContent - leftWidth - 4
		if maxCenter > 0 {
			centerText = TruncateVisible(centerText, maxCenter)
		} else {
			centerText = ""
		}
	}
	centerWidth := VisibleWidth(centerText)

	usedByLeftCenter := leftWidth + centerWidth
	if centerWidth > 0 {
		usedByLeftCenter += 2
	}
	availableForRight := maxContent - usedByLeftCenter - 1

	rightWidth := 0
	if rightText != "" {
		rightWidth = VisibleWidth(rightText)
		if availableForRight <= 0 {
			rightText = ""
			rightWidth = 0
		} else if rightWidth > availableForRight {
			rightText = TruncateFromStart(rightText, availableForRight)
			rightWidth = VisibleWidth(rightText)
		}
	}

	centerCol := 0
	if centerText != "" {
		c := (maxContent - centerWidth) / 2
		if c < leftWidth+1 {
			c = leftWidth + 1
		}
		centerCol = c
	}

	rightCol := maxContent
	if rightText != "" {
		rightCol = maxContent - rightWidth
	}

	buf.WriteString(leftText)
	currentPos := leftWidth

	if centerText != "" {
		gapToCenter := centerCol - currentPos
		if gapToCenter > 0 {
			buf.WriteString(strings.Repeat(" ", gapToCenter))
		}
		buf.WriteString(centerText)
		currentPos = centerCol + centerWidth
	}

	fillEnd := rightCol
	if rightText == "" {
		fillEnd = maxContent
	}
	gap := fillEnd - currentPos
	if gap > 0 {
		buf.WriteString(strings.Repeat(" ", gap))
	}

	if rightText != "" {
		buf.WriteString(rightText)
		buf.WriteString(ResetFG)
	}

	buf.WriteString(Reset)
	if trailingNewline {
		buf.WriteByte('\n')
	}
}

func (l *Line) Render(buf *strings.Builder, width int) {
	l.render(buf, width, true)
}

func (l *Line) RenderNoNewline(buf *strings.Builder, width int) {
	l.render(buf, width, false)
}

type Screen struct {
	Header     *Section
	Body       *Section
	Footer     *Section
	Input      *InputField
	width      int
	height     int
	fixedWidth int
	fixedHeight int
	writer     io.Writer
}

func NewScreen(w io.Writer, fixedWidth, fixedHeight int) *Screen {
	return &Screen{
		Header:      NewSection(),
		Body:        NewSection(),
		Footer:      NewSection(),
		writer:      w,
		fixedWidth:  fixedWidth,
		fixedHeight: fixedHeight,
	}
}

func (s *Screen) SetInput(placeholder, text string, cursor int) *InputField {
	s.Input = NewInputField(placeholder, text, cursor)
	return s.Input
}

func (s *Screen) refreshSize() {
	if s.fixedWidth > 0 && s.fixedHeight > 0 {
		s.width = s.fixedWidth
		s.height = s.fixedHeight
		return
	}
	rows, cols := TerminalSize()
	if s.fixedWidth > 0 {
		s.width = s.fixedWidth
	} else {
		s.width = cols
	}
	if s.fixedHeight > 0 {
		s.height = s.fixedHeight
	} else {
		s.height = rows
	}
}

func (s *Screen) Clear() {
	s.Header.Clear()
	s.Body.Clear()
	s.Footer.Clear()
	s.Input = nil
}

func (s *Screen) Flush() {
	s.refreshSize()

	var buf strings.Builder
	buf.WriteString(Home)

	cursorRow := 0
	cursorCol := 0
	currentRow := 0

	for _, line := range s.Header.Lines {
		if s.Input != nil && line.HasInput() {
			cursorRow = currentRow + 1
			cursorCol = line.CursorColumn(s.Input, s.width)
		}
		line.Render(&buf, s.width)
		currentRow++
	}

	footerLines := len(s.Footer.Lines)
	bodySpace := s.height - currentRow - footerLines

	bodyRendered := 0
	for _, line := range s.Body.Lines {
		if bodyRendered >= bodySpace {
			break
		}
		if s.Input != nil && line.HasInput() {
			cursorRow = currentRow + 1
			cursorCol = line.CursorColumn(s.Input, s.width)
		}
		line.Render(&buf, s.width)
		currentRow++
		bodyRendered++
	}

	gap := bodySpace - bodyRendered
	blankLine := fmt.Sprintf("\r%s%s\n", ClearEOL, strings.Repeat(" ", s.width-1))
	blankLineNoNewline := fmt.Sprintf("\r%s%s", ClearEOL, strings.Repeat(" ", s.width-1))
	for i := 0; i < gap; i++ {
		if i == gap-1 && len(s.Footer.Lines) == 0 {
			buf.WriteString(blankLineNoNewline)
		} else {
			buf.WriteString(blankLine)
		}
		currentRow++
	}

	for i, line := range s.Footer.Lines {
		if s.Input != nil && line.HasInput() {
			cursorRow = currentRow + 1
			cursorCol = line.CursorColumn(s.Input, s.width)
		}
		if i == len(s.Footer.Lines)-1 {
			line.RenderNoNewline(&buf, s.width)
		} else {
			line.Render(&buf, s.width)
		}
		currentRow++
	}

	if cursorRow > 0 && cursorCol > 0 && s.Input != nil {
		fmt.Fprintf(&buf, "\x1b[%d;%dH", cursorRow, cursorCol)
		buf.WriteString(ShowCursor)
	} else {
		buf.WriteString(HideCursor)
	}

	buf.WriteString(Reset)

	io.WriteString(s.writer, buf.String())

	s.Clear()
}
