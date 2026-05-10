package selector

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/ollietreend/go-try/internal/fuzzy"
	"github.com/ollietreend/go-try/internal/tui"
)

func (s *bubbleSelector) View() string {
	switch s.mode {
	case ModeDelete:
		return s.viewDelete()
	case ModeRename:
		return s.viewRename()
	case ModeAscend:
		return s.viewAscend()
	case ModePrompt:
		return s.viewPrompt()
	default:
		return s.viewMain()
	}
}

func (s *bubbleSelector) viewMain() string {
	w := s.width
	if w < 20 {
		w = 80
	}
	h := s.height
	if h < 10 {
		h = 24
	}

	var buf strings.Builder

	buf.WriteString(centerLine(w, "\U0001F3E0 "+tui.AccentText("Try Directory Selection")))
	buf.WriteByte('\n')
	buf.WriteString(tui.DimText(lineFill(w, "\u2500")))
	buf.WriteByte('\n')

	prefix := "Search: "
	inputText := renderInput(s.inputBuf, s.inputCursor)
	buf.WriteString(tui.DimText(prefix) + inputText)
	buf.WriteByte('\n')

	buf.WriteString(tui.DimText(lineFill(w, "\u2500")))
	buf.WriteByte('\n')

	results := s.getResults()
	showCreateNew := s.inputBuf != ""
	visible := h - 6
	if visible < 3 {
		visible = 3
	}

	totalItems := len(results)
	if showCreateNew {
		totalItems++
	}

	visibleEnd := s.scrollOffset + visible
	if visibleEnd > totalItems {
		visibleEnd = totalItems
	}

	for idx := s.scrollOffset; idx < visibleEnd; idx++ {
		if idx < len(results) {
			buf.WriteString(s.renderEntryLine(results[idx], idx == s.cursorPos, w))
		} else if showCreateNew && idx == len(results) {
			buf.WriteString(s.renderCreateLine(idx == s.cursorPos, w))
		}
		buf.WriteByte('\n')
	}

	remaining := visible - (visibleEnd - s.scrollOffset)
	for i := 0; i < remaining; i++ {
		buf.WriteByte('\n')
	}

	buf.WriteString(tui.DimText(lineFill(w, "\u2500")))
	buf.WriteByte('\n')

	if s.deleteMode {
		buf.WriteString(tui.BoldText(fmt.Sprintf(" DELETE MODE  %d marked  |  Ctrl-D: Toggle  Enter: Confirm  Esc: Cancel", len(s.markedForDeletion))))
	} else {
		buf.WriteString(tui.DimText("\u2191/\u2193: Navigate  Enter: Select  ^R: Rename  ^G: Graduate  ^D: Delete  Esc: Cancel"))
	}

	return buf.String()
}

func (s *bubbleSelector) renderEntryLine(result fuzzy.MatchResult, selected bool, w int) string {
	entry := result.Entry
	isMarked := false
	for _, p := range s.markedForDeletion {
		if p == entry.Path {
			isMarked = true
			break
		}
	}

	arrow := "  "
	if selected {
		arrow = "\u2192 "
	}

	icon := "\U0001F4C1"
	if isMarked {
		icon = "\U0001F5D1\uFE0F"
	} else if entry.IsSymlink {
		icon = "\U0001F517"
	}

	name := entry.Name
	var nameStr string
	if fuzzy.HasDatePrefix(name) {
		datePart := name[:10]
		namePart := name[11:]
		if len(result.Positions) > 0 {
			nameStr = tui.DimText(datePart+"-") + highlightPositions(namePart, result.Positions, 11)
		} else {
			nameStr = tui.DimText(datePart+"-") + namePart
		}
	} else {
		if len(result.Positions) > 0 {
			nameStr = highlightPositions(name, result.Positions, 0)
		} else {
			nameStr = name
		}
	}

	leftContent := arrow + icon + " " + nameStr
	rightContent := fmt.Sprintf("%s, %.1f", formatRelativeTime(entry.Mtime), result.Score)

	fullLine := padRight(leftContent, tui.DimText(rightContent), w)

	if isMarked {
		fullLine = "\x1b[48;5;52m" + fullLine + "\x1b[49m"
	} else if selected {
		fullLine = "\x1b[48;5;238m" + fullLine + "\x1b[49m"
	}

	return fullLine
}

func (s *bubbleSelector) renderCreateLine(selected bool, w int) string {
	arrow := "  "
	if selected {
		arrow = "\u2192 "
	}
	datePrefix := time.Now().Format("2006-01-02")
	label := fmt.Sprintf("\U0001F4C2 Create new: %s-%s", datePrefix, s.inputBuf)
	if s.inputBuf == "" {
		label = fmt.Sprintf("\U0001F4C2 Create new: %s-", datePrefix)
	}
	line := arrow + label
	line = padRight(line, "", w)
	if selected {
		line = "\x1b[48;5;238m" + line + "\x1b[49m"
	}
	return line
}

func (s *bubbleSelector) viewDelete() string {
	w := s.width
	if w < 30 {
		w = 80
	}

	var buf strings.Builder

	results := s.getResults()
	var markedItems []fuzzy.MatchResult
	for _, r := range results {
		for _, mp := range s.markedForDeletion {
			if r.Entry.Path == mp {
				markedItems = append(markedItems, r)
				break
			}
		}
	}

	count := len(markedItems)
	buf.WriteString(centerLine(w, "\U0001F5D1\uFE0F "+tui.AccentText(fmt.Sprintf("Delete %d %s?", count, pluralWord("directory", count)))))
	buf.WriteByte('\n')
	buf.WriteString(tui.DimText(lineFill(w, "\u2500")))
	buf.WriteByte('\n')

	for _, item := range markedItems {
		buf.WriteString("  \U0001F5D1\uFE0F " + item.Entry.Name + "\n")
	}

	buf.WriteByte('\n')
	buf.WriteByte('\n')

	prefix := "Type YES to confirm: "
	inputText := renderInput(s.deleteBuf, s.deleteCursor)
	buf.WriteString(centerLine(w, tui.DimText(prefix)+inputText))
	buf.WriteByte('\n')

	buf.WriteByte('\n')
	buf.WriteString(tui.DimText(lineFill(w, "\u2500")))
	buf.WriteByte('\n')
	buf.WriteString(centerLine(w, tui.DimText("Enter: Confirm  Esc: Cancel")))

	return buf.String()
}

func (s *bubbleSelector) viewRename() string {
	w := s.width
	if w < 30 {
		w = 80
	}

	var buf strings.Builder

	buf.WriteString(centerLine(w, "\u270F\uFE0F "+tui.AccentText("Rename directory")))
	buf.WriteByte('\n')
	buf.WriteString(tui.DimText(lineFill(w, "\u2500")))
	buf.WriteByte('\n')

	buf.WriteString("\U0001F4C1 " + s.renameEntry.Entry.Name + "\n")
	buf.WriteByte('\n')
	buf.WriteByte('\n')

	prefix := "New name: "
	inputText := renderInput(s.renameBuf, s.renameCursor)
	buf.WriteString(centerLine(w, tui.DimText(prefix)+inputText))
	buf.WriteByte('\n')

	if s.renameError != "" {
		buf.WriteByte('\n')
		buf.WriteString(centerLine(w, tui.BoldText(s.renameError)))
		buf.WriteByte('\n')
	}

	buf.WriteByte('\n')
	buf.WriteString(tui.DimText(lineFill(w, "\u2500")))
	buf.WriteByte('\n')
	buf.WriteString(centerLine(w, tui.DimText("Enter: Confirm  Esc: Cancel")))

	return buf.String()
}

func (s *bubbleSelector) viewAscend() string {
	w := s.width
	if w < 30 {
		w = 80
	}

	var buf strings.Builder

	buf.WriteString(centerLine(w, "\U0001F680 "+tui.AccentText("Graduate try to project")))
	buf.WriteByte('\n')
	buf.WriteString(tui.DimText(lineFill(w, "\u2500")))
	buf.WriteByte('\n')

	buf.WriteString("\U0001F4C1 " + s.ascendEntry.Entry.Name + "\n")
	buf.WriteByte('\n')

	envHint := "parent of $TRY_PATH"
	if s.ascendProjectsDir != filepath.Dir(s.basePath) {
		envHint = "$TRY_PROJECTS"
	}
	buf.WriteString(centerLine(w, tui.DimText(fmt.Sprintf("Destination (%s: %s)", envHint, s.ascendProjectsDir))))
	buf.WriteByte('\n')

	prefix := "Move to: "
	inputText := renderInput(s.ascendBuf, s.ascendCursor)
	buf.WriteString(centerLine(w, tui.DimText(prefix)+inputText))
	buf.WriteByte('\n')

	buf.WriteByte('\n')
	buf.WriteString(centerLine(w, tui.DimText("A symlink will be left in the tries directory")))
	buf.WriteByte('\n')

	if s.ascendError != "" {
		buf.WriteByte('\n')
		buf.WriteString(centerLine(w, tui.BoldText(s.ascendError)))
		buf.WriteByte('\n')
	}

	buf.WriteByte('\n')
	buf.WriteString(tui.DimText(lineFill(w, "\u2500")))
	buf.WriteByte('\n')
	buf.WriteString(centerLine(w, tui.DimText("Enter: Confirm  Esc: Cancel")))

	return buf.String()
}

func (s *bubbleSelector) viewPrompt() string {
	w := s.width
	if w < 30 {
		w = 80
	}

	var buf strings.Builder

	datePrefix := time.Now().Format("2006-01-02")
	buf.WriteString("Enter new try name\n\n")
	buf.WriteString("> " + datePrefix + "-" + renderInput(s.promptBuf, s.promptCursor) + "\n")

	return buf.String()
}

func renderInput(text string, cursor int) string {
	f := tui.NewInputField("", text, cursor)
	return f.String()
}

func padRight(left, right string, width int) string {
	leftW := tui.VisibleWidth(left)
	rightW := tui.VisibleWidth(right)
	maxContent := width - 1
	if maxContent < 1 {
		maxContent = 1
	}

	if leftW+rightW+1 > maxContent {
		available := maxContent - rightW - 1
		if available < 5 {
			return left + " " + right
		}
		left = tui.Truncate(left, available, "\u2026")
		leftW = tui.VisibleWidth(left)
	}

	padding := maxContent - leftW - rightW
	if padding < 1 {
		padding = 1
	}

	return left + strings.Repeat(" ", padding) + right
}

func centerLine(w int, content string) string {
	visWidth := tui.VisibleWidth(content)
	leftPad := (w - 1 - visWidth) / 2
	if leftPad < 0 {
		leftPad = 0
	}
	return strings.Repeat(" ", leftPad) + content
}

func lineFill(w int, char string) string {
	mw := w - 1
	if mw < 1 {
		mw = 1
	}
	return strings.Repeat(char, mw)
}

func keyStringToMsg(k string) tea.KeyMsg {
	switch k {
	case "\r", "\n":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "\x1b":
		return tea.KeyMsg{Type: tea.KeyEsc}
	case "\x1b[A":
		return tea.KeyMsg{Type: tea.KeyUp}
	case "\x1b[B":
		return tea.KeyMsg{Type: tea.KeyDown}
	case "\x7f", "\b":
		return tea.KeyMsg{Type: tea.KeyBackspace}
	case "\x01":
		return tea.KeyMsg{Type: tea.KeyCtrlA}
	case "\x02":
		return tea.KeyMsg{Type: tea.KeyCtrlB}
	case "\x04":
		return tea.KeyMsg{Type: tea.KeyCtrlD}
	case "\x05":
		return tea.KeyMsg{Type: tea.KeyCtrlE}
	case "\x06":
		return tea.KeyMsg{Type: tea.KeyCtrlF}
	case "\x07":
		return tea.KeyMsg{Type: tea.KeyCtrlG}
	case "\x0b":
		return tea.KeyMsg{Type: tea.KeyCtrlK}
	case "\x0e":
		return tea.KeyMsg{Type: tea.KeyCtrlN}
	case "\x10":
		return tea.KeyMsg{Type: tea.KeyCtrlP}
	case "\x12":
		return tea.KeyMsg{Type: tea.KeyCtrlR}
	case "\x14":
		return tea.KeyMsg{Type: tea.KeyCtrlT}
	case "\x17":
		return tea.KeyMsg{Type: tea.KeyCtrlW}
	default:
		if len(k) == 1 {
			return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(k)}
		}
		return tea.KeyMsg{}
	}
}

func pluralWord(w string, n int) string {
	if n == 1 {
		return w
	}
	return w + "s"
}

func RunBubbletea(opts ...Option) *Result {
	s := &bubbleSelector{basePath: TRY_PATH}
	for _, o := range opts {
		o.applyBubble(s)
	}
	if s.basePath == "" {
		if p := os.Getenv("TRY_PATH"); p != "" {
			s.basePath = p
		}
	}
	if s.inputBuf == "" && s.searchTerm != "" {
		s.inputBuf = strings.ReplaceAll(s.searchTerm, " ", "-")
		s.inputCursor = len(s.inputBuf)
	}
	s.width = 80
	s.height = 24

	progOpts := []tea.ProgramOption{
		tea.WithoutSignalHandler(),
		tea.WithOutput(os.Stderr),
	}

	renderOnce := s.renderOnce
	testKeys := s.testKeys

	if renderOnce && (testKeys == nil || len(testKeys) == 0) {
		s.init()
		os.Stderr.WriteString(s.View())
		return nil
	}

	if testKeys != nil && len(testKeys) > 0 {
		s.init()
		for _, k := range testKeys {
			msg := keyStringToMsg(k)
			model, cmd := s.Update(msg)
			next, ok := model.(*bubbleSelector)
			if !ok {
				break
			}
			s = next
			if cmd != nil {
				break
			}
		}
		if renderOnce {
			os.Stderr.WriteString(s.View())
		}
		return s.result
	}

	p := tea.NewProgram(s, progOpts...)
	model, err := p.Run()
	if err != nil {
		return nil
	}
	if m, ok := model.(*bubbleSelector); ok {
		return m.result
	}
	return nil
}
