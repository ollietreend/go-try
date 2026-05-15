package selector

import (
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/bubbles/textinput"

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

	searchInput textinput.Model
	cursorPos   int
	scrollOffset int
	allTries    []DirEntry
	matcher     *fuzzy.Matcher
	lastQuery   string
	cachedResults []fuzzy.MatchResult

	deleteMode  bool
	markedForDeletion []string

	dialogInput   textinput.Model

	renameEntry    fuzzy.MatchResult
	renameError    string

	ascendEntry    fuzzy.MatchResult
	ascendError    string
	ascendProjectsDir string

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

	ti := textinput.New()
	ti.Placeholder = ""
	ti.CharLimit = 0
	ti.Width = 60
	ti.Focus()
	s.searchInput = ti

	s.dialogInput = textinput.New()
	s.dialogInput.CharLimit = 0
	s.dialogInput.Width = 60
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
	query := s.searchInput.Value()
	if s.lastQuery == query && s.cachedResults != nil {
		return s.cachedResults
	}
	s.lastQuery = query
	if s.matcher == nil {
		return nil
	}
	maxResults := s.height - 6
	if maxResults < 3 {
		maxResults = 3
	}
	all := s.matcher.Match(query)
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
	s.clampCursor()
	s.clampScroll()
	return s, nil
}

func (s *bubbleSelector) clampCursor() {
	results := s.getResults()
	total := len(results)
	if s.searchInput.Value() != "" {
		total++
	}
	if s.cursorPos >= total {
		s.cursorPos = total - 1
	}
	if s.cursorPos < 0 {
		s.cursorPos = 0
	}
}

func (s *bubbleSelector) clampScroll() {
	results := s.getResults()
	showCreateNew := s.searchInput.Value() != ""
	total := len(results)
	if showCreateNew {
		total++
	}
	visible := s.height - 6
	if visible < 3 {
		visible = 3
	}
	if s.cursorPos < s.scrollOffset {
		s.scrollOffset = s.cursorPos
	}
	if s.cursorPos >= s.scrollOffset+visible {
		s.scrollOffset = s.cursorPos - visible + 1
	}
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
	// Let textinput handle text-editing keys first
	prev := s.searchInput.Value()
	ti, _ := s.searchInput.Update(msg)
	s.searchInput = ti
	edited := s.searchInput.Value() != prev
	if edited {
		s.cursorPos = 0
		return s, nil
	}

	switch msg.Type {
	case tea.KeyEnter:
		query := s.searchInput.Value()
		results := s.getResults()
		if s.deleteMode && len(s.markedForDeletion) > 0 {
			s.mode = ModeDelete
			s.dialogInput.SetValue("")
			s.dialogInput.Focus()
			return s, nil
		} else if s.cursorPos < len(results) {
			s.result = &Result{Type: ResultCd, Path: results[s.cursorPos].Entry.Path}
			return s, tea.Quit
		} else if query != "" {
			datePrefix := time.Now().Format("2006-01-02")
			name := datePrefix + "-" + strings.ReplaceAll(query, " ", "-")
			s.result = &Result{Type: ResultMkdir, Path: filepath.Join(s.basePath, name)}
			return s, tea.Quit
		} else {
			s.mode = ModePrompt
			s.dialogInput.SetValue("")
			s.dialogInput.Focus()
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
		query := s.searchInput.Value()
		total := len(s.getResults())
		if query != "" {
			total++
		}
		if s.cursorPos < total-1 {
			s.cursorPos++
		}

	case tea.KeyCtrlD:
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

	case tea.KeyCtrlG:
		results := s.getResults()
		if s.cursorPos < len(results) {
			s.enterAscendDialog(results[s.cursorPos])
		}

	case tea.KeyCtrlN:
		query := s.searchInput.Value()
		total := len(s.getResults())
		if query != "" {
			total++
		}
		if s.cursorPos < total-1 {
			s.cursorPos++
		}

	case tea.KeyCtrlP:
		if s.cursorPos > 0 {
			s.cursorPos--
		}

	case tea.KeyCtrlR:
		results := s.getResults()
		if s.cursorPos < len(results) {
			s.enterRenameDialog(results[s.cursorPos])
		}

	case tea.KeyCtrlT:
		query := s.searchInput.Value()
		datePrefix := time.Now().Format("2006-01-02")
		if query != "" {
			name := datePrefix + "-" + strings.ReplaceAll(query, " ", "-")
			s.result = &Result{Type: ResultMkdir, Path: filepath.Join(s.basePath, name)}
		} else {
			s.mode = ModePrompt
			s.dialogInput.SetValue("")
			s.dialogInput.Focus()
		}
		if s.result != nil {
			return s, tea.Quit
		}

	}
	return s, nil
}

func inputCharRune(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') ||
		(r >= '0' && r <= '9') || r == '-' || r == '_' || r == '.' || r == ' '
}

func (s *bubbleSelector) handleDeleteKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	ti, _ := s.dialogInput.Update(msg)
	s.dialogInput = ti

	if msg.Type == tea.KeyEnter {
		if s.dialogInput.Value() == "YES" {
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
			s.markedForDeletion = nil
			s.deleteMode = false
			s.mode = ModeMain
			s.searchInput.Focus()
			return s, tea.Quit
		}
		s.markedForDeletion = nil
		s.deleteMode = false
		s.mode = ModeMain
		s.searchInput.Focus()
	}
	if msg.Type == tea.KeyEsc {
		s.markedForDeletion = nil
		s.deleteMode = false
		s.mode = ModeMain
		s.searchInput.Focus()
	}
	return s, nil
}

func (s *bubbleSelector) enterRenameDialog(entry fuzzy.MatchResult) {
	s.deleteMode = false
	s.markedForDeletion = nil
	s.renameEntry = entry
	s.renameError = ""
	s.dialogInput.SetValue(entry.Entry.Name)
	s.dialogInput.Focus()
	s.mode = ModeRename
}

func (s *bubbleSelector) handleRenameKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	ti, _ := s.dialogInput.Update(msg)
	s.dialogInput = ti

	if msg.Type == tea.KeyEnter {
		errMsg := s.finalizeRename()
		if errMsg == "" {
			if s.result != nil {
				return s, tea.Quit
			}
			s.searchInput.Focus()
			return s, nil
		}
		s.renameError = errMsg
	}
	if msg.Type == tea.KeyEsc {
		s.mode = ModeMain
		s.searchInput.Focus()
	}
	return s, nil
}

func renameCharRune(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') ||
		(r >= '0' && r <= '9') || r == '-' || r == '_' || r == '.' || r == ' ' || r == '/'
}

func (s *bubbleSelector) finalizeRename() string {
	newName := strings.TrimSpace(s.dialogInput.Value())
	newName = strings.ReplaceAll(newName, " ", "-")
	if newName == "" {
		return "Name cannot be empty"
	}
	if strings.Contains(newName, "/") {
		return "Name cannot contain /"
	}
	if newName == s.renameEntry.Entry.Name {
		s.mode = ModeMain
		s.result = nil
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
	s.ascendError = ""
	s.dialogInput.SetValue(filepath.Join(s.ascendProjectsDir, projectName))
	s.dialogInput.Focus()
	s.mode = ModeAscend
}

func (s *bubbleSelector) handleAscendKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	ti, _ := s.dialogInput.Update(msg)
	s.dialogInput = ti

	if msg.Type == tea.KeyEnter {
		errMsg := s.finalizeAscend()
		if errMsg == "" {
			return s, tea.Quit
		}
		s.ascendError = errMsg
	}
	if msg.Type == tea.KeyEsc {
		s.mode = ModeMain
		s.searchInput.Focus()
	}
	return s, nil
}

func (s *bubbleSelector) finalizeAscend() string {
	dest := strings.TrimSpace(s.dialogInput.Value())
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
	ti, _ := s.dialogInput.Update(msg)
	s.dialogInput = ti

	if msg.Type == tea.KeyEnter {
		if s.dialogInput.Value() != "" {
			datePrefix := time.Now().Format("2006-01-02")
			name := datePrefix + "-" + strings.ReplaceAll(s.dialogInput.Value(), " ", "-")
			s.result = &Result{Type: ResultMkdir, Path: filepath.Join(s.basePath, name)}
			return s, tea.Quit
		}
		s.mode = ModeMain
		s.searchInput.Focus()
	}
	if msg.Type == tea.KeyEsc {
		s.mode = ModeMain
		s.searchInput.Focus()
	}
	return s, nil
}
