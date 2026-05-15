package selector

import (
	"fmt"
	"math"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"golang.org/x/term"

	"github.com/ollietreend/go-try/internal/fuzzy"
	"github.com/ollietreend/go-try/internal/tui"
)

var TRY_PATH = ""

func init() {
	if p := os.Getenv("TRY_PATH"); p != "" {
		TRY_PATH = p
	} else {
		if home, err := os.UserHomeDir(); err == nil {
			TRY_PATH = filepath.Join(home, "src", "tries")
		}
	}
}

type DirEntry struct {
	Name      string
	Path      string
	Mtime     int64
	IsSymlink bool

	fuzzyEntry fuzzy.Entry
}

type ResultType int

const (
	ResultCd ResultType = iota
	ResultMkdir
	ResultDelete
	ResultRename
	ResultAscend
)

type Result struct {
	Type     ResultType
	Path     string
	Paths    []string
	BasePath string
	OldName  string
	NewName  string
	Source   string
	Dest     string
	Basename string
}

type Selector struct {
	basePath           string
	inputBuffer        string
	cursorPos          int
	inputCursorPos     int
	scrollOffset       int
	selected           *Result
	allTries           []DirEntry
	matcher            *fuzzy.Matcher
	cachedResults      []fuzzy.MatchResult
	lastQuery          string
	deleteMode         bool
	markedForDeletion  []string
	deleteStatus       string
	needsRedraw        bool
	width              int
	height             int
	testRenderOnce     bool
	testNoCls          bool
	testKeys           []string
	testConfirm        string
	andType            string
	rawMode            bool
	oldTermState       *term.State
	keyReaderDone      chan struct{}
	keyCh              chan string
	oldWinch           chan os.Signal
	winch              chan os.Signal
}

type Option func(*Selector)

func WithTestRenderOnce() Option {
	return func(s *Selector) { s.testRenderOnce = true }
}

func WithTestNoCls() Option {
	return func(s *Selector) { s.testNoCls = true }
}

func WithTestKeys(keys []string) Option {
	return func(s *Selector) { s.testKeys = keys }
}

func WithTestConfirm(c string) Option {
	return func(s *Selector) { s.testConfirm = c }
}

func WithAndType(t string) Option {
	return func(s *Selector) { s.andType = t }
}

func WithBasePath(p string) Option {
	return func(s *Selector) { s.basePath = p }
}

func WithSearchTerm(term string) Option {
	return func(s *Selector) {
		s.inputBuffer = strings.ReplaceAll(term, " ", "-")
		s.inputCursorPos = len(s.inputBuffer)
	}
}

func NewSelector(opts ...Option) *Selector {
	s := &Selector{
		basePath: TRY_PATH,
	}
	for _, o := range opts {
		o(s)
	}
	if s.andType != "" {
		s.inputBuffer = strings.ReplaceAll(s.andType, " ", "-")
		s.inputCursorPos = len(s.inputBuffer)
	}
	return s
}

func (s *Selector) Run() *Result {
	if s.basePath == "" {
		fmt.Fprintf(os.Stderr, "Error: TRY_PATH not set\n")
		return nil
	}
	os.MkdirAll(s.basePath, 0755)

	s.loadTries()

	if s.testRenderOnce && (s.testKeys == nil || len(s.testKeys) == 0) {
		h, w := tui.TerminalSize()
		s.width = w
		s.height = h
		s.render()
		return nil
	}

	hasTTY := term.IsTerminal(int(os.Stdin.Fd())) && term.IsTerminal(int(os.Stderr.Fd()))
	if !hasTTY {
		if s.testKeys == nil || len(s.testKeys) == 0 {
			fmt.Fprintln(os.Stderr, "Error: try requires an interactive terminal")
			return nil
		}
		s.runLoop()
		return s.selected
	}

	s.setupTerminal()
	defer s.restoreTerminal()
	s.runLoop()
	return s.selected
}

func (s *Selector) setupTerminal() {
	if !s.testNoCls {
		os.Stderr.WriteString(tui.AltScreenOn)
		os.Stderr.WriteString(tui.SetTitle("try"))
	}

	oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err == nil {
		s.rawMode = true
		s.oldTermState = oldState
	}

	s.keyCh = make(chan string, 32)
	s.keyReaderDone = make(chan struct{})
	go s.keyReader()

	s.winch = make(chan os.Signal, 1)
	signal.Notify(s.winch, syscall.SIGWINCH)

	h, w := tui.TerminalSize()
	s.width = w
	s.height = h
}

func (s *Selector) restoreTerminal() {
	if s.keyReaderDone != nil {
		close(s.keyReaderDone)
	}
	if s.winch != nil {
		signal.Stop(s.winch)
	}
	if s.rawMode {
		term.Restore(int(os.Stdin.Fd()), s.oldTermState)
	}

	if !s.testNoCls {
		os.Stderr.WriteString(tui.Reset)
		os.Stderr.WriteString(tui.AltScreenOff)
	}
}

func (s *Selector) keyReader() {
	buf := make([]byte, 10)
	for {
		select {
		case <-s.keyReaderDone:
			return
		default:
		}
		n, err := os.Stdin.Read(buf)
		if err != nil {
			select {
			case <-s.keyReaderDone:
				return
			case s.keyCh <- "\x03":
			}
			return
		}
		if n > 0 {
			data := make([]byte, n)
			copy(data, buf[:n])
			select {
			case <-s.keyReaderDone:
				return
			case s.keyCh <- string(data):
			}
		}
	}
}

func (s *Selector) readKey() string {
	if s.testKeys != nil && len(s.testKeys) > 0 {
		k := s.testKeys[0]
		s.testKeys = s.testKeys[1:]
		return k
	}
	if s.testKeys != nil && len(s.testKeys) == 0 {
		return "\x1b"
	}

	for {
		if s.needsRedraw {
			s.needsRedraw = false
			return ""
		}
		select {
		case key := <-s.keyCh:
			return key
		case <-s.winch:
			s.needsRedraw = true
			h, w := tui.TerminalSize()
			s.width = w
			s.height = h
			return ""
		case <-time.After(100 * time.Millisecond):
		}
	}
}

func (s *Selector) loadTries() {
	entries, err := os.ReadDir(s.basePath)
	if err != nil {
		return
	}

	now := time.Now().Unix()
	var tries []DirEntry

	for _, e := range entries {
		name := e.Name()
		if strings.HasPrefix(name, ".") {
			continue
		}

		fullPath := filepath.Join(s.basePath, name)
		isSymlink := e.Type()&os.ModeSymlink != 0

		var mtime int64
		var isDir bool

		if isSymlink {
			if resolved, err := filepath.EvalSymlinks(fullPath); err == nil {
				fullPath = resolved
				if info, err := os.Stat(resolved); err == nil {
					mtime = info.ModTime().Unix()
					isDir = info.IsDir()
				}
			}
		} else {
			info, err := e.Info()
			if err == nil {
				mtime = info.ModTime().Unix()
				isDir = info.IsDir()
			}
		}

		if !isDir {
			continue
		}
		hoursSinceAccess := float64(now-mtime) / 3600.0
		baseScore := 3.0 / math.Sqrt(hoursSinceAccess+1)

		if fuzzy.HasDatePrefix(name) {
			baseScore += 2.0
		}

		tries = append(tries, DirEntry{
			Name:      name,
			Path:      fullPath,
			Mtime:     mtime,
			IsSymlink: isSymlink,
			fuzzyEntry: fuzzy.Entry{
				Name:      name,
				Path:      fullPath,
				Mtime:     mtime,
				IsSymlink: isSymlink,
				BaseScore: baseScore,
			},
		})
	}

	s.allTries = tries
	s.buildMatcher()
}

func (s *Selector) buildMatcher() {
	entries := make([]fuzzy.Entry, len(s.allTries))
	for i, t := range s.allTries {
		entries[i] = t.fuzzyEntry
	}
	s.matcher = fuzzy.New(entries)
}

func (s *Selector) getResults() []fuzzy.MatchResult {
	if s.lastQuery == s.inputBuffer && s.cachedResults != nil {
		return s.cachedResults
	}
	s.lastQuery = s.inputBuffer

	if s.matcher == nil {
		return nil
	}

	maxResults := s.height - 6
	if maxResults < 3 {
		maxResults = 3
	}

	all := s.matcher.Match(s.inputBuffer)

	if len(all) > maxResults {
		sortResults(all)
		all = all[:maxResults]
	} else {
		sortResults(all)
	}

	s.cachedResults = all
	return all
}

func sortResults(results []fuzzy.MatchResult) {
	for i := 0; i < len(results); i++ {
		for j := i + 1; j < len(results); j++ {
			if results[j].Score > results[i].Score {
				results[i], results[j] = results[j], results[i]
			}
		}
	}
}

func (s *Selector) runLoop() {
	for {
		results := s.getResults()
		showCreateNew := s.inputBuffer != ""
		totalItems := len(results) + 0
		if showCreateNew {
			totalItems++
		}

		if s.cursorPos >= totalItems {
			s.cursorPos = totalItems - 1
		}
		if s.cursorPos < 0 {
			s.cursorPos = 0
		}

		s.render()

		key := s.readKey()
		if key == "" {
			continue
		}

		switch key {
		case "\r":
			if s.deleteMode && len(s.markedForDeletion) > 0 {
				s.confirmBatchDelete(results)
				if s.selected != nil {
					return
				}
			} else if s.cursorPos < len(results) {
				s.handleSelect(results[s.cursorPos])
				if s.selected != nil {
					return
				}
			} else if showCreateNew {
				s.handleCreateNew()
				if s.selected != nil {
					return
				}
			}
		case "\x1b[A", "\x10":
			if s.cursorPos > 0 {
				s.cursorPos--
			}
		case "\x1b[B", "\x0e":
			if s.cursorPos < totalItems-1 {
				s.cursorPos++
			}
		case "\x7f", "\b":
			if s.inputCursorPos > 0 {
				s.inputBuffer = s.inputBuffer[:s.inputCursorPos-1] + s.inputBuffer[s.inputCursorPos:]
				s.inputCursorPos--
			}
			s.cursorPos = 0
		case "\x01":
			s.inputCursorPos = 0
		case "\x05":
			s.inputCursorPos = len(s.inputBuffer)
		case "\x02":
			if s.inputCursorPos > 0 {
				s.inputCursorPos--
			}
		case "\x06":
			if s.inputCursorPos < len(s.inputBuffer) {
				s.inputCursorPos++
			}
		case "\x0b":
			s.inputBuffer = s.inputBuffer[:s.inputCursorPos]
		case "\x17":
			if s.inputCursorPos > 0 {
				newPos := wordBoundaryBackward(s.inputBuffer, s.inputCursorPos)
				s.inputBuffer = s.inputBuffer[:newPos] + s.inputBuffer[s.inputCursorPos:]
				s.inputCursorPos = newPos
			}
		case "\x04":
			if s.cursorPos < len(results) {
				path := results[s.cursorPos].Entry.Path
				found := false
				for i, p := range s.markedForDeletion {
					if p == path {
						s.markedForDeletion = append(s.markedForDeletion[:i], s.markedForDeletion[i+1:]...)
						found = true
						break
					}
				}
				if !found {
					s.markedForDeletion = append(s.markedForDeletion, path)
					s.deleteMode = true
				}
				if len(s.markedForDeletion) == 0 {
					s.deleteMode = false
				}
			}
		case "\x14":
			s.handleCreateNew()
			if s.selected != nil {
				return
			}
		case "\x12":
			if s.cursorPos < len(results) {
				s.runRenameDialog(results[s.cursorPos])
				if s.selected != nil {
					return
				}
			}
		case "\x07":
			if s.cursorPos < len(results) {
				s.runAscendDialog(results[s.cursorPos])
				if s.selected != nil {
					return
				}
			}
		case "\x03", "\x1b":
			if s.deleteMode {
				s.markedForDeletion = nil
				s.deleteMode = false
			} else {
				return
			}
		default:
			if len(key) == 1 && inputChar(key[0]) {
				before := s.inputBuffer[:s.inputCursorPos]
				after := s.inputBuffer[s.inputCursorPos:]
				s.inputBuffer = before + key + after
				s.inputCursorPos++
				s.cursorPos = 0
			}
		}
	}
}

func inputChar(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') ||
		(b >= '0' && b <= '9') || b == '-' || b == '_' || b == '.' || b == ' '
}

func wordBoundaryBackward(s string, cursor int) int {
	pos := cursor - 1
	for pos >= 0 && !isWordChar(s[pos]) {
		pos--
	}
	for pos >= 0 && isWordChar(s[pos]) {
		pos--
	}
	return pos + 1
}

func isWordChar(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9')
}

func (s *Selector) handleSelect(result fuzzy.MatchResult) {
	s.selected = &Result{
		Type: ResultCd,
		Path: result.Entry.Path,
	}
}

func (s *Selector) handleCreateNew() {
	datePrefix := time.Now().Format("2006-01-02")

	if s.inputBuffer != "" {
		name := datePrefix + "-" + strings.ReplaceAll(s.inputBuffer, " ", "-")
		s.selected = &Result{
			Type: ResultMkdir,
			Path: filepath.Join(s.basePath, name),
		}
		return
	}

	s.restoreTerminal()
	fmt.Fprintln(os.Stderr, "Enter new try name")
	fmt.Fprintln(os.Stderr)
	fmt.Fprintf(os.Stderr, "> %s-", datePrefix)

	var entry string
	fmt.Scanln(&entry)

	s.setupTerminal()

	if entry == "" {
		return
	}

	name := datePrefix + "-" + strings.ReplaceAll(entry, " ", "-")
	s.selected = &Result{
		Type: ResultMkdir,
		Path: filepath.Join(s.basePath, name),
	}
}

func (s *Selector) confirmBatchDelete(results []fuzzy.MatchResult) {
	var markedItems []fuzzy.MatchResult
	for _, r := range results {
		for _, p := range s.markedForDeletion {
			if r.Entry.Path == p {
				markedItems = append(markedItems, r)
				break
			}
		}
	}
	if len(markedItems) == 0 {
		return
	}

	if !s.testNoCls {
		s.clearScreen()
	}

	confirmBuf := ""
	confirmCursor := 0

	if s.testKeys != nil && len(s.testKeys) > 0 {
		for len(s.testKeys) > 0 {
			ch := s.testKeys[0]
			s.testKeys = s.testKeys[1:]
			if ch == "\r" || ch == "\n" {
				break
			}
			confirmBuf += ch
			confirmCursor = len(confirmBuf)
		}
		s.processDeleteConfirm(markedItems, confirmBuf)
		return
	}

	for {
		if !s.testNoCls {
			s.clearScreen()
		}
		s.renderDeleteDialog(markedItems, confirmBuf, confirmCursor)

		key := s.readKey()
		if key == "" {
			continue
		}
		switch key {
		case "\r":
			s.processDeleteConfirm(markedItems, confirmBuf)
			return
		case "\x1b":
			s.deleteStatus = "Delete cancelled"
			s.markedForDeletion = nil
			s.deleteMode = false
			return
		case "\x7f", "\b":
			if confirmCursor > 0 {
				confirmBuf = confirmBuf[:confirmCursor-1] + confirmBuf[confirmCursor:]
				confirmCursor--
			}
		case "\x03":
			s.deleteStatus = "Delete cancelled"
			s.markedForDeletion = nil
			s.deleteMode = false
			return
		default:
			if len(key) == 1 && key[0] >= 32 {
				confirmBuf = confirmBuf[:confirmCursor] + key + confirmBuf[confirmCursor:]
				confirmCursor++
			}
		}
	}
}

func (s *Selector) processDeleteConfirm(items []fuzzy.MatchResult, confirmation string) {
	if confirmation != "YES" {
		s.deleteStatus = "Delete cancelled"
		s.markedForDeletion = nil
		s.deleteMode = false
		return
	}

	var validatedPaths []string
	var names []string
	baseReal, _ := filepath.EvalSymlinks(s.basePath)

	for _, item := range items {
		targetReal, err := filepath.EvalSymlinks(item.Entry.Path)
		if err != nil || !strings.HasPrefix(targetReal, baseReal+"/") {
			continue
		}
		validatedPaths = append(validatedPaths, item.Entry.Name)
		names = append(names, item.Entry.Name)
	}

	if len(validatedPaths) == 0 {
		return
	}

	s.deleteStatus = "Deleted: " + strings.Join(names, ", ")
	s.selected = &Result{
		Type:     ResultDelete,
		Paths:    validatedPaths,
		BasePath: baseReal,
	}
	s.markedForDeletion = nil
	s.deleteMode = false
}

func (s *Selector) clearScreen() {
	os.Stderr.WriteString("\x1b[2J\x1b[H")
}

func (s *Selector) render() {
	scr := tui.NewScreen(os.Stderr, s.width, s.height)

	scr.Header.AddLine("").Write().WriteEmoji("\U0001F3E0").WriteAccent(" Try Directory Selection")
	scr.Header.AddLine("").Write().WriteDim(fill("─", s.width))

	searchLine := scr.Header.AddLine("")
	prefix := "Search: "
	searchLine.Write().WriteDim(prefix)
	scr.SetInput("", s.inputBuffer, s.inputCursorPos)
	searchLine.Write().Write(scr.Input.String())
	searchLine.MarkHasInput(len(prefix))

	scr.Header.AddLine("").Write().WriteDim(fill("─", s.width))

	results := s.getResults()
	showCreateNew := s.inputBuffer != ""
	totalItems := len(results)
	if showCreateNew {
		totalItems++
	}

	if s.cursorPos >= s.scrollOffset+maxVisible(s) {
		s.scrollOffset = s.cursorPos - maxVisible(s) + 1
	}
	if s.cursorPos < s.scrollOffset {
		s.scrollOffset = s.cursorPos
	}

	visibleEnd := s.scrollOffset + maxVisible(s)
	if visibleEnd > totalItems {
		visibleEnd = totalItems
	}

	for idx := s.scrollOffset; idx < visibleEnd; idx++ {
		if idx < len(results) {
			s.renderEntryLine(scr, results[idx], idx == s.cursorPos)
		} else if showCreateNew && idx == len(results) {
			s.renderCreateLine(scr, idx == s.cursorPos)
		}
	}

	scr.Footer.AddLine("").Write().WriteDim(fill("─", s.width))
	if s.deleteStatus != "" {
		scr.Footer.AddLine("").Write().WriteBold(s.deleteStatus)
		s.deleteStatus = ""
	} else if s.deleteMode {
		scr.Footer.AddLine(tui.DangerBG).Write().WriteBold(" DELETE MODE ").Write(fmt.Sprintf(" %d marked  |  Ctrl-D: Toggle  Enter: Confirm  Esc: Cancel", len(s.markedForDeletion)))
	} else {
		scr.Footer.AddLine("").Write().WriteDim("\u2191/\u2193: Navigate  Enter: Select  ^R: Rename  ^G: Graduate  ^D: Delete  Esc: Cancel")
	}

	scr.Flush()
}

func maxVisible(s *Selector) int {
	mv := s.height - 6
	if mv < 3 {
		mv = 3
	}
	return mv
}

func (s *Selector) renderEntryLine(scr *tui.Screen, result fuzzy.MatchResult, selected bool) {
	entry := result.Entry
	isMarked := false
	for _, p := range s.markedForDeletion {
		if p == entry.Path {
			isMarked = true
			break
		}
	}

	bg := ""
	if isMarked {
		bg = tui.DangerBG
	} else if selected {
		bg = tui.SelectedBG
	}

	line := scr.Body.AddLine(bg)

	arrow := "  "
	if selected {
		arrow = "\u2192 "
	}
	line.Write().Write(arrow)

	icon := "\U0001F4C1"
	if isMarked {
		icon = "\U0001F5D1"
	} else if entry.IsSymlink {
		icon = "\U0001F517"
	}
	if isMarked {
		line.Write().WriteEmoji(icon).Write("  ")
	} else {
		line.Write().WriteEmoji(icon).Write(" ")
	}

	name := entry.Name
	if fuzzy.HasDatePrefix(name) {
		datePart := name[:10]
		namePart := name[11:]
		line.Write().WriteDim(datePart + "-")
		if len(result.Positions) > 0 {
			line.Write().Write(highlightPositions(namePart, result.Positions, 11))
		} else {
			line.Write().Write(namePart)
		}
	} else {
		if len(result.Positions) > 0 {
			line.Write().Write(highlightPositions(name, result.Positions, 0))
		} else {
			line.Write().Write(name)
		}
	}

	meta := fmt.Sprintf("%s, %.1f", formatRelativeTime(entry.Mtime), result.Score)
	line.Right().WriteDim(meta)
}

func highlightPositions(text string, positions []int, offset int) string {
	posSet := make(map[int]bool)
	for _, p := range positions {
		posSet[p-offset] = true
	}

	var buf strings.Builder
	runes := []rune(text)
	highlighting := false

	for i, r := range runes {
		if posSet[i] && !highlighting {
			buf.WriteString(tui.Highlight)
			highlighting = true
		} else if !posSet[i] && highlighting {
			buf.WriteString(tui.ResetFG + tui.ResetIntensity)
			highlighting = false
		}
		buf.WriteRune(r)
	}
	if highlighting {
		buf.WriteString(tui.ResetFG + tui.ResetIntensity)
	}
	return buf.String()
}

func (s *Selector) renderCreateLine(scr *tui.Screen, selected bool) {
	bg := ""
	if selected {
		bg = tui.SelectedBG
	}
	line := scr.Body.AddLine(bg)
	arrow := "  "
	if selected {
		arrow = "\u2192 "
	}
	line.Write().Write(arrow)
	datePrefix := time.Now().Format("2006-01-02")
	label := fmt.Sprintf("\U0001F4C2 Create new: %s-%s", datePrefix, s.inputBuffer)
	if s.inputBuffer == "" {
		label = fmt.Sprintf("\U0001F4C2 Create new: %s-", datePrefix)
	}
	line.Write().Write(label)
}

func (s *Selector) renderDeleteDialog(items []fuzzy.MatchResult, confirmBuf string, confirmCursor int) {
	scr := tui.NewScreen(os.Stderr, s.width, s.height)

	scr.Header.AddLine("").Center().WriteEmoji("\U0001F5D1").WriteAccent(fmt.Sprintf("  Delete %d %s?", len(items), plural("directory", len(items))))
	scr.Header.AddLine("").Write().WriteDim(fill("─", s.width))

	for _, item := range items {
		scr.Body.AddLine(tui.DangerBG).Write().WriteEmoji("\U0001F5D1").Write(" " + item.Entry.Name)
	}

	scr.Body.AddLine("")
	scr.Body.AddLine("")

	confirmLine := scr.Body.AddLine("")
	prefix := "Type YES to confirm: "
	confirmLine.Center().WriteDim(prefix)
	scr.SetInput("", confirmBuf, confirmCursor)
	confirmLine.Center().Write(scr.Input.String())
	confirmLine.MarkHasInput((s.width-len(prefix)-(len(confirmBuf)+1))/2 + len(prefix))

	scr.Footer.AddLine("").Write().WriteDim(fill("─", s.width))
	scr.Footer.AddLine("").Center().WriteDim("Enter: Confirm  Esc: Cancel")

	scr.Flush()
}

func (s *Selector) runRenameDialog(result fuzzy.MatchResult) {
	entry := result.Entry
	s.deleteMode = false
	s.markedForDeletion = nil

	currentName := entry.Name
	renameBuf := currentName
	renameCursor := len(renameBuf)
	renameError := ""

	for {
		s.renderRenameDialog(currentName, renameBuf, renameCursor, renameError)

		key := s.readKey()
		if key == "" {
			continue
		}
		switch key {
		case "\r":
			errMsg := s.finalizeRename(entry.Name, renameBuf)
			if errMsg == "" {
				return
			}
			renameError = errMsg
		case "\x1b", "\x03":
			return
		case "\x7f", "\b":
			if renameCursor > 0 {
				renameBuf = renameBuf[:renameCursor-1] + renameBuf[renameCursor:]
				renameCursor--
			}
			renameError = ""
		case "\x01":
			renameCursor = 0
		case "\x05":
			renameCursor = len(renameBuf)
		case "\x02":
			if renameCursor > 0 {
				renameCursor--
			}
		case "\x06":
			if renameCursor < len(renameBuf) {
				renameCursor++
			}
		case "\x0b":
			renameBuf = renameBuf[:renameCursor]
			renameError = ""
		case "\x17":
			if renameCursor > 0 {
				newPos := wordBoundaryBackward(renameBuf, renameCursor)
				renameBuf = renameBuf[:newPos] + renameBuf[renameCursor:]
				renameCursor = newPos
			}
			renameError = ""
		default:
			if len(key) == 1 && renameChar(key[0]) {
				renameBuf = renameBuf[:renameCursor] + key + renameBuf[renameCursor:]
				renameCursor++
				renameError = ""
			}
		}
	}
}

func renameChar(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') ||
		(b >= '0' && b <= '9') || b == '-' || b == '_' || b == '.' || b == ' ' || b == '/'
}

func (s *Selector) finalizeRename(oldName, newName string) string {
	newName = strings.TrimSpace(newName)
	newName = strings.ReplaceAll(newName, " ", "-")

	if newName == "" {
		return "Name cannot be empty"
	}
	if strings.Contains(newName, "/") {
		return "Name cannot contain /"
	}
	if newName == oldName {
		s.selected = nil
		return ""
	}
	if _, err := os.Stat(filepath.Join(s.basePath, newName)); err == nil {
		return "Directory exists: " + newName
	}

	s.selected = &Result{
		Type:     ResultRename,
		BasePath: s.basePath,
		OldName:  oldName,
		NewName:  newName,
	}
	return ""
}

func (s *Selector) renderRenameDialog(currentName, renameBuf string, renameCursor int, renameError string) {
	scr := tui.NewScreen(os.Stderr, s.width, s.height)

	scr.Header.AddLine("").Center().WriteEmoji("\u270F\uFE0F").WriteAccent("  Rename directory")
	scr.Header.AddLine("").Write().WriteDim(fill("─", s.width))

	scr.Body.AddLine("").Write().WriteEmoji("\U0001F4C1").Write(" " + currentName)
	scr.Body.AddLine("")
	scr.Body.AddLine("")

	renameLine := scr.Body.AddLine("")
	prefix := "New name: "
	renameLine.Center().WriteDim(prefix)
	scr.SetInput("", renameBuf, renameCursor)
	renameLine.Center().Write(scr.Input.String())
	inputWidth := len(renameBuf) + 1
	if inputWidth < renameCursor+1 {
		inputWidth = renameCursor + 1
	}
	centerStart := (s.width - len(prefix) - inputWidth) / 2
	renameLine.MarkHasInput(centerStart + len(prefix))

	if renameError != "" {
		scr.Body.AddLine("")
		scr.Body.AddLine("").Center().WriteBold(renameError)
	}

	scr.Footer.AddLine("").Write().WriteDim(fill("─", s.width))
	scr.Footer.AddLine("").Center().WriteDim("Enter: Confirm  Esc: Cancel")

	scr.Flush()
}

func (s *Selector) runAscendDialog(result fuzzy.MatchResult) {
	entry := result.Entry
	s.deleteMode = false
	s.markedForDeletion = nil

	currentName := entry.Name
	projectName := currentName
	if fuzzy.HasDatePrefix(projectName) && len(projectName) > 11 {
		projectName = projectName[11:]
	}

	projectsDir := filepath.Dir(s.basePath)
	if p := os.Getenv("TRY_PROJECTS"); p != "" {
		projectsDir = p
	}

	ascendBuf := filepath.Join(projectsDir, projectName)
	ascendCursor := len(ascendBuf)
	ascendError := ""

	for {
		s.renderAscendDialog(currentName, ascendBuf, ascendCursor, ascendError, projectsDir)

		key := s.readKey()
		if key == "" {
			continue
		}
		switch key {
		case "\r":
			errMsg := s.finalizeAscend(entry.Name, entry.Path, ascendBuf)
			if errMsg == "" {
				return
			}
			ascendError = errMsg
		case "\x1b", "\x03":
			return
		case "\x7f", "\b":
			if ascendCursor > 0 {
				ascendBuf = ascendBuf[:ascendCursor-1] + ascendBuf[ascendCursor:]
				ascendCursor--
			}
			ascendError = ""
		case "\x01":
			ascendCursor = 0
		case "\x05":
			ascendCursor = len(ascendBuf)
		case "\x02":
			if ascendCursor > 0 {
				ascendCursor--
			}
		case "\x06":
			if ascendCursor < len(ascendBuf) {
				ascendCursor++
			}
		case "\x0b":
			ascendBuf = ascendBuf[:ascendCursor]
			ascendError = ""
		case "\x17":
			if ascendCursor > 0 {
				newPos := wordBoundaryBackward(ascendBuf, ascendCursor)
				ascendBuf = ascendBuf[:newPos] + ascendBuf[ascendCursor:]
				ascendCursor = newPos
			}
			ascendError = ""
		default:
			if len(key) == 1 && ascendChar(key[0]) {
				ascendBuf = ascendBuf[:ascendCursor] + key + ascendBuf[ascendCursor:]
				ascendCursor++
				ascendError = ""
			}
		}
	}
}

func ascendChar(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') ||
		(b >= '0' && b <= '9') || b == '-' || b == '_' || b == '.' || b == ' ' || b == '/' || b == '~'
}

func (s *Selector) finalizeAscend(basename, source, dest string) string {
	dest = strings.TrimSpace(dest)

	if dest == "" {
		return "Destination cannot be empty"
	}
	if _, err := os.Stat(dest); err == nil {
		return "Destination already exists: " + dest
	}

	parent := filepath.Dir(dest)
	if _, err := os.Stat(parent); os.IsNotExist(err) {
		return "Parent directory does not exist: " + parent
	}

	s.selected = &Result{
		Type:     ResultAscend,
		Source:   source,
		Dest:     dest,
		Basename: basename,
		BasePath: s.basePath,
	}
	return ""
}

func (s *Selector) renderAscendDialog(currentName, ascendBuf string, ascendCursor int, ascendError, projectsDir string) {
	scr := tui.NewScreen(os.Stderr, s.width, s.height)

	scr.Header.AddLine("").Center().WriteEmoji("\U0001F680").WriteAccent("  Graduate try to project")
	scr.Header.AddLine("").Write().WriteDim(fill("─", s.width))

	scr.Body.AddLine("").Write().WriteEmoji("\U0001F4C1").Write(" " + currentName)
	scr.Body.AddLine("")

	envHint := "parent of $TRY_PATH"
	if os.Getenv("TRY_PROJECTS") != "" {
		envHint = "$TRY_PROJECTS"
	}
	scr.Body.AddLine("").Center().WriteDim(fmt.Sprintf("Destination (%s: %s)", envHint, projectsDir))

	ascendLine := scr.Body.AddLine("")
	prefix := "Move to: "
	ascendLine.Center().WriteDim(prefix)
	scr.SetInput("", ascendBuf, ascendCursor)
	ascendLine.Center().Write(scr.Input.String())
	inputWidth := len(ascendBuf) + 1
	if inputWidth < ascendCursor+1 {
		inputWidth = ascendCursor + 1
	}
	centerStart := (s.width - len(prefix) - inputWidth) / 2
	ascendLine.MarkHasInput(centerStart + len(prefix))

	scr.Body.AddLine("")
	scr.Body.AddLine("").Center().WriteDim("A symlink will be left in the tries directory")

	if ascendError != "" {
		scr.Body.AddLine("")
		scr.Body.AddLine("").Center().WriteBold(ascendError)
	}

	scr.Footer.AddLine("").Write().WriteDim(fill("─", s.width))
	scr.Footer.AddLine("").Center().WriteDim("Enter: Confirm  Esc: Cancel")

	scr.Flush()
}

func formatRelativeTime(mtime int64) string {
	now := time.Now().Unix()
	seconds := now - mtime

	if seconds < 60 {
		return "just now"
	}
	minutes := seconds / 60
	if minutes < 60 {
		return fmt.Sprintf("%dm ago", minutes)
	}
	hours := minutes / 60
	if hours < 24 {
		return fmt.Sprintf("%dh ago", hours)
	}
	days := hours / 24
	if days < 7 {
		return fmt.Sprintf("%dd ago", days)
	}
	return fmt.Sprintf("%dw ago", days/7)
}

func fill(char string, width int) string {
	c := "─"
	if char != "" {
		c = char
	}
	if width <= 1 {
		return c
	}
	return strings.Repeat(c, width-2)
}

func plural(word string, n int) string {
	if n == 1 {
		return word
	}
	return word + "s"
}
