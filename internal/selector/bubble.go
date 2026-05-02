package selector

import (
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/ollietreend/go-try/internal/fuzzy"
)

type Mode int

const (
	ModeMain    Mode = iota
	ModePrompt
	ModeDelete
	ModeRename
	ModeAscend
)

type bubbleSelector struct {
	mode Mode

	width  int
	height int

	basePath   string
	searchTerm string
	result     *Result

	inputBuf    string
	inputCursor int
	cursorPos   int
	scrollOffset int
	allTries    []DirEntry
	matcher     *fuzzy.Matcher
	lastQuery   string
	cachedResults []fuzzy.MatchResult

	deleteMode  bool
	markedForDeletion []string
	deleteBuf   string
	deleteCursor int

	renameEntry    fuzzy.MatchResult
	renameBuf      string
	renameCursor   int
	renameError    string

	ascendEntry    fuzzy.MatchResult
	ascendBuf      string
	ascendCursor   int
	ascendError    string
	ascendProjectsDir string

	promptBuf    string
	promptCursor int

	renderOnce bool
	noCls      bool
	testKeys   []string
}

func (opt Option) applyBubble(s *bubbleSelector) {
	tmp := &Selector{}
	opt(tmp)
	if tmp.basePath != "" {
		s.basePath = tmp.basePath
	}
	if tmp.inputBuffer != "" {
		s.searchTerm = tmp.inputBuffer
	}
	if tmp.testRenderOnce {
		s.renderOnce = true
	}
	if tmp.testNoCls {
		s.noCls = true
	}
	if tmp.testKeys != nil {
		s.testKeys = tmp.testKeys
	}
}

func (s *bubbleSelector) init() {
	os.MkdirAll(s.basePath, 0755)
	s.loadTries()
}

func (s *bubbleSelector) loadTries() {
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
	fe := make([]fuzzy.Entry, len(tries))
	for i, t := range tries {
		fe[i] = t.fuzzyEntry
	}
	s.matcher = fuzzy.New(fe)
}

func (s *bubbleSelector) getResults() []fuzzy.MatchResult {
	if s.lastQuery == s.inputBuf && s.cachedResults != nil {
		return s.cachedResults
	}
	s.lastQuery = s.inputBuf
	if s.matcher == nil {
		return nil
	}
	maxResults := s.height - 6
	if maxResults < 3 {
		maxResults = 3
	}
	all := s.matcher.Match(s.inputBuf)
	sortResults(all)
	if len(all) > maxResults {
		all = all[:maxResults]
	}
	s.cachedResults = all
	return all
}

func (s *bubbleSelector) Init() tea.Cmd {
	s.init()
	return nil
}

func (s *bubbleSelector) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		s.width = msg.Width
		s.height = msg.Height
	case tea.KeyMsg:
		return s.handleKey(msg)
	}
	return s, nil
}

func (s *bubbleSelector) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch s.mode {
	case ModeMain:
		return s.handleMainKey(msg)
	case ModeDelete:
		return s.handleDeleteKey(msg)
	case ModeRename:
		return s.handleRenameKey(msg)
	case ModeAscend:
		return s.handleAscendKey(msg)
	case ModePrompt:
		return s.handlePromptKey(msg)
	}
	return s, nil
}

func (s *bubbleSelector) handleMainKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEnter:
		results := s.getResults()
		if s.deleteMode && len(s.markedForDeletion) > 0 {
			s.mode = ModeDelete
			s.deleteBuf = ""
			s.deleteCursor = 0
			return s, nil
		} else if s.cursorPos < len(results) {
			s.result = &Result{Type: ResultCd, Path: results[s.cursorPos].Entry.Path}
			return s, tea.Quit
		} else if s.inputBuf != "" {
			datePrefix := time.Now().Format("2006-01-02")
			name := datePrefix + "-" + strings.ReplaceAll(s.inputBuf, " ", "-")
			s.result = &Result{Type: ResultMkdir, Path: filepath.Join(s.basePath, name)}
			return s, tea.Quit
		} else {
			s.mode = ModePrompt
			s.promptBuf = ""
			s.promptCursor = 0
			return s, nil
		}

	case tea.KeyEsc:
		if s.deleteMode {
			s.markedForDeletion = nil
			s.deleteMode = false
		} else {
			return s, tea.Quit
		}

	case tea.KeyUp:
		if s.cursorPos > 0 {
			s.cursorPos--
		}

	case tea.KeyDown:
		total := len(s.getResults())
		if s.inputBuf != "" {
			total++
		}
		if s.cursorPos < total-1 {
			s.cursorPos++
		}

	case tea.KeyBackspace:
		if s.inputCursor > 0 {
			s.inputBuf = s.inputBuf[:s.inputCursor-1] + s.inputBuf[s.inputCursor:]
			s.inputCursor--
		}
		s.cursorPos = 0

	case tea.KeyRunes:
		for _, r := range msg.Runes {
			switch r {
			case 1:
				s.inputCursor = 0
			case 2:
				if s.inputCursor > 0 {
					s.inputCursor--
				}
			case 4:
				results := s.getResults()
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
			case 5:
				s.inputCursor = len(s.inputBuf)
			case 6:
				if s.inputCursor < len(s.inputBuf) {
					s.inputCursor++
				}
			case 7:
				results := s.getResults()
				if s.cursorPos < len(results) {
					s.enterAscendDialog(results[s.cursorPos])
				}
			case 11:
				s.inputBuf = s.inputBuf[:s.inputCursor]
			case 14:
				total := len(s.getResults())
				if s.inputBuf != "" {
					total++
				}
				if s.cursorPos < total-1 {
					s.cursorPos++
				}
			case 16:
				if s.cursorPos > 0 {
					s.cursorPos--
				}
			case 18:
				results := s.getResults()
				if s.cursorPos < len(results) {
					s.enterRenameDialog(results[s.cursorPos])
				}
			case 20:
				datePrefix := time.Now().Format("2006-01-02")
				if s.inputBuf != "" {
					name := datePrefix + "-" + strings.ReplaceAll(s.inputBuf, " ", "-")
					s.result = &Result{Type: ResultMkdir, Path: filepath.Join(s.basePath, name)}
				} else {
					s.mode = ModePrompt
				}
				if s.result != nil {
					return s, tea.Quit
				}
			case 23:
				if s.inputCursor > 0 {
					newPos := wordBoundaryBackward(s.inputBuf, s.inputCursor)
					s.inputBuf = s.inputBuf[:newPos] + s.inputBuf[s.inputCursor:]
					s.inputCursor = newPos
				}
			default:
				if r >= 32 && r != 127 && inputCharRune(r) {
					s.inputBuf = s.inputBuf[:s.inputCursor] + string(r) + s.inputBuf[s.inputCursor:]
					s.inputCursor++
					s.cursorPos = 0
				}
			}
		}
	}
	return s, nil
}

func inputCharRune(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') ||
		(r >= '0' && r <= '9') || r == '-' || r == '_' || r == '.' || r == ' '
}

func (s *bubbleSelector) handleDeleteKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEnter:
		if s.deleteBuf == "YES" {
			results := s.getResults()
			var validatedPaths []string
			baseReal, _ := filepath.EvalSymlinks(s.basePath)
			for _, r := range results {
				for _, mp := range s.markedForDeletion {
					if r.Entry.Path == mp {
						_, err := os.Stat(r.Entry.Path)
						if err == nil {
							validatedPaths = append(validatedPaths, r.Entry.Name)
						}
						break
					}
				}
			}
			s.result = &Result{
				Type:     ResultDelete,
				Paths:    validatedPaths,
				BasePath: baseReal,
			}
		}
		s.markedForDeletion = nil
		s.deleteMode = false
		s.mode = ModeMain
		return s, tea.Quit
	case tea.KeyEsc:
		s.markedForDeletion = nil
		s.deleteMode = false
		s.mode = ModeMain
	case tea.KeyBackspace:
		if s.deleteCursor > 0 {
			s.deleteBuf = s.deleteBuf[:s.deleteCursor-1] + s.deleteBuf[s.deleteCursor:]
			s.deleteCursor--
		}
	case tea.KeyRunes:
		for _, r := range msg.Runes {
			if r >= 32 {
				s.deleteBuf = s.deleteBuf[:s.deleteCursor] + string(r) + s.deleteBuf[s.deleteCursor:]
				s.deleteCursor++
			}
		}
	}
	return s, nil
}

func (s *bubbleSelector) enterRenameDialog(entry fuzzy.MatchResult) {
	s.deleteMode = false
	s.markedForDeletion = nil
	s.renameEntry = entry
	s.renameBuf = entry.Entry.Name
	s.renameCursor = len(s.renameBuf)
	s.renameError = ""
	s.mode = ModeRename
}

func (s *bubbleSelector) handleRenameKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEnter:
		errMsg := s.finalizeRename()
		if errMsg == "" {
			return s, tea.Quit
		}
		s.renameError = errMsg
	case tea.KeyEsc:
		s.mode = ModeMain
	case tea.KeyBackspace:
		if s.renameCursor > 0 {
			s.renameBuf = s.renameBuf[:s.renameCursor-1] + s.renameBuf[s.renameCursor:]
			s.renameCursor--
		}
		s.renameError = ""
	case tea.KeyRunes:
		for _, r := range msg.Runes {
			switch r {
			case 1:
				s.renameCursor = 0
			case 5:
				s.renameCursor = len(s.renameBuf)
			case 2:
				if s.renameCursor > 0 {
					s.renameCursor--
				}
			case 6:
				if s.renameCursor < len(s.renameBuf) {
					s.renameCursor++
				}
			case 11:
				s.renameBuf = s.renameBuf[:s.renameCursor]
				s.renameError = ""
			case 23:
				if s.renameCursor > 0 {
					newPos := wordBoundaryBackward(s.renameBuf, s.renameCursor)
					s.renameBuf = s.renameBuf[:newPos] + s.renameBuf[s.renameCursor:]
					s.renameCursor = newPos
				}
				s.renameError = ""
			default:
				if r >= 32 && renameCharRune(r) {
					s.renameBuf = s.renameBuf[:s.renameCursor] + string(r) + s.renameBuf[s.renameCursor:]
					s.renameCursor++
					s.renameError = ""
				}
			}
		}
	}
	return s, nil
}

func renameCharRune(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') ||
		(r >= '0' && r <= '9') || r == '-' || r == '_' || r == '.' || r == ' ' || r == '/'
}

func (s *bubbleSelector) finalizeRename() string {
	newName := strings.TrimSpace(s.renameBuf)
	newName = strings.ReplaceAll(newName, " ", "-")
	if newName == "" {
		return "Name cannot be empty"
	}
	if strings.Contains(newName, "/") {
		return "Name cannot contain /"
	}
	if newName == s.renameEntry.Entry.Name {
		s.mode = ModeMain
		return ""
	}
	if _, err := os.Stat(filepath.Join(s.basePath, newName)); err == nil {
		return "Directory exists: " + newName
	}
	s.result = &Result{
		Type:     ResultRename,
		BasePath: s.basePath,
		OldName:  s.renameEntry.Entry.Name,
		NewName:  newName,
	}
	return ""
}

func (s *bubbleSelector) enterAscendDialog(entry fuzzy.MatchResult) {
	s.deleteMode = false
	s.markedForDeletion = nil
	s.ascendEntry = entry
	projectName := entry.Entry.Name
	if fuzzy.HasDatePrefix(projectName) && len(projectName) > 11 {
		projectName = projectName[11:]
	}
	s.ascendProjectsDir = filepath.Dir(s.basePath)
	if p := os.Getenv("TRY_PROJECTS"); p != "" {
		s.ascendProjectsDir = p
	}
	s.ascendBuf = filepath.Join(s.ascendProjectsDir, projectName)
	s.ascendCursor = len(s.ascendBuf)
	s.ascendError = ""
	s.mode = ModeAscend
}

func (s *bubbleSelector) handleAscendKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEnter:
		errMsg := s.finalizeAscend()
		if errMsg == "" {
			return s, tea.Quit
		}
		s.ascendError = errMsg
	case tea.KeyEsc:
		s.mode = ModeMain
	case tea.KeyBackspace:
		if s.ascendCursor > 0 {
			s.ascendBuf = s.ascendBuf[:s.ascendCursor-1] + s.ascendBuf[s.ascendCursor:]
			s.ascendCursor--
		}
		s.ascendError = ""
	case tea.KeyRunes:
		for _, r := range msg.Runes {
			switch r {
			case 1:
				s.ascendCursor = 0
			case 5:
				s.ascendCursor = len(s.ascendBuf)
			case 2:
				if s.ascendCursor > 0 {
					s.ascendCursor--
				}
			case 6:
				if s.ascendCursor < len(s.ascendBuf) {
					s.ascendCursor++
				}
			case 11:
				s.ascendBuf = s.ascendBuf[:s.ascendCursor]
				s.ascendError = ""
			case 23:
				if s.ascendCursor > 0 {
					newPos := wordBoundaryBackward(s.ascendBuf, s.ascendCursor)
					s.ascendBuf = s.ascendBuf[:newPos] + s.ascendBuf[s.ascendCursor:]
					s.ascendCursor = newPos
				}
				s.ascendError = ""
			default:
				if r >= 32 && ascendCharRune(r) {
					s.ascendBuf = s.ascendBuf[:s.ascendCursor] + string(r) + s.ascendBuf[s.ascendCursor:]
					s.ascendCursor++
					s.ascendError = ""
				}
			}
		}
	}
	return s, nil
}

func ascendCharRune(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') ||
		(r >= '0' && r <= '9') || r == '-' || r == '_' || r == '.' || r == ' ' || r == '/' || r == '~'
}

func (s *bubbleSelector) finalizeAscend() string {
	dest := strings.TrimSpace(s.ascendBuf)
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
	s.result = &Result{
		Type:     ResultAscend,
		Source:   s.ascendEntry.Entry.Path,
		Dest:     dest,
		Basename: s.ascendEntry.Entry.Name,
		BasePath: s.basePath,
	}
	return ""
}

func (s *bubbleSelector) handlePromptKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEnter:
		if s.promptBuf != "" {
			datePrefix := time.Now().Format("2006-01-02")
			name := datePrefix + "-" + strings.ReplaceAll(s.promptBuf, " ", "-")
			s.result = &Result{Type: ResultMkdir, Path: filepath.Join(s.basePath, name)}
			return s, tea.Quit
		}
		s.mode = ModeMain
	case tea.KeyEsc:
		s.mode = ModeMain
	case tea.KeyBackspace:
		if s.promptCursor > 0 {
			s.promptBuf = s.promptBuf[:s.promptCursor-1] + s.promptBuf[s.promptCursor:]
			s.promptCursor--
		}
	case tea.KeyRunes:
		for _, r := range msg.Runes {
			if r >= 32 && inputCharRune(r) {
				s.promptBuf = s.promptBuf[:s.promptCursor] + string(r) + s.promptBuf[s.promptCursor:]
				s.promptCursor++
			}
		}
	}
	return s, nil
}
