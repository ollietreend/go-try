package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/ollietreend/go-try/internal/selector"
	"github.com/ollietreend/go-try/internal/shell"
	"github.com/ollietreend/go-try/internal/tui"
)

const version = "0.1.0"

func main() {
	processColorFlags()
	processGlobalFlags()

	// Extract --path option
	triesPath := extractPathOption()
	if triesPath == "" {
		triesPath = selector.TRY_PATH
	}

	// Test-only flags
	andType := extractOptionWithValue("--and-type")
	andExit := removeFlag("--and-exit")
	andKeysRaw := extractOptionWithValue("--and-keys")
	andConfirm := extractOptionWithValue("--and-confirm")
	andKeys := parseTestKeys(andKeysRaw)

	if len(os.Args) < 2 {
		printHelp()
		os.Exit(2)
	}

	command := os.Args[1]
	rest := os.Args[2:]

	switch command {
	case "clone":
		cmds := cmdClone(rest, triesPath)
		shell.EmitScript(cmds)
	case "init":
		cmdInit(rest, triesPath)
	case "install":
		cmdInstall(rest, triesPath)
	case "exec":
		if len(rest) == 0 {
			result := runSelector(triesPath, "", andType, andExit, andKeys, andConfirm)
			if result != nil {
				emitResult(*result)
			} else {
				fmt.Println("Cancelled.")
				os.Exit(1)
			}
			return
		}
		sub := rest[0]
		subArgs := rest[1:]
		switch sub {
		case "clone":
			cmds := cmdClone(subArgs, triesPath)
			shell.EmitScript(cmds)
		case "worktree":
			cmds := cmdWorktree(subArgs, triesPath)
			shell.EmitScript(cmds)
		case "cd":
			query := strings.Join(subArgs, " ")
			if isGitURI(strings.Split(query, " ")[0]) {
				emitCloneURI(query, triesPath)
				return
			}
			result := runSelector(triesPath, query, andType, andExit, andKeys, andConfirm)
			if result != nil {
				emitResult(*result)
			} else {
				fmt.Println("Cancelled.")
				os.Exit(1)
			}
		case ".":
			handleDotShorthand(".", subArgs, triesPath)
		default:
			query := strings.Join(rest, " ")
			if isGitURI(strings.Split(query, " ")[0]) {
				emitCloneURI(query, triesPath)
				return
			}
			if strings.HasPrefix(sub, ".") {
				handleDotShorthand(sub, subArgs, triesPath)
				return
			}
			result := runSelector(triesPath, query, andType, andExit, andKeys, andConfirm)
			if result != nil {
				emitResult(*result)
			} else {
				fmt.Println("Cancelled.")
				os.Exit(1)
			}
		}
	case "worktree":
		cmds := cmdWorktree(rest, triesPath)
		shell.EmitScript(cmds)
	case "cd":
		result := runSelector(triesPath, strings.Join(rest, " "), andType, andExit, andKeys, andConfirm)
		if result != nil {
			emitResult(*result)
		} else {
			fmt.Println("Cancelled.")
			os.Exit(1)
		}
	default:
		query := strings.Join(os.Args[1:], " ")
		first := ""
		if len(os.Args[1:]) > 0 {
			first = os.Args[1]
		}

		if isGitURI(strings.Split(query, " ")[0]) {
			emitCloneURI(query, triesPath)
			return
		}
		if strings.HasPrefix(first, ".") {
			handleDotShorthand(first, rest, triesPath)
			return
		}
		result := runSelector(triesPath, query, andType, andExit, andKeys, andConfirm)
		if result != nil {
			emitResult(*result)
		} else {
			fmt.Println("Cancelled.")
			os.Exit(1)
		}
	}
}

func processColorFlags() {
	for i, a := range os.Args[1:] {
		if a == "--no-colors" || a == "--no-expand-tokens" {
			tui.ColorsEnabled = false
			os.Args = append(os.Args[:i+1], os.Args[i+2:]...)
			return
		}
	}
	if os.Getenv("NO_COLOR") != "" {
		tui.ColorsEnabled = false
	}
}

func processGlobalFlags() {
	for _, a := range os.Args[1:] {
		if a == "--help" || a == "-h" {
			printHelp()
			os.Exit(0)
		}
		if a == "--version" || a == "-v" {
			fmt.Fprintf(os.Stderr, "try %s\n", version)
			os.Exit(0)
		}
	}
}

func extractPathOption() string {
	for i, a := range os.Args[1:] {
		if a == "--path" && i+2 < len(os.Args) {
			val := os.Args[i+2]
			os.Args = append(os.Args[:i+1], os.Args[i+3:]...)
			return filepath.Clean(val)
		}
		if strings.HasPrefix(a, "--path=") {
			val := strings.TrimPrefix(a, "--path=")
			os.Args = append(os.Args[:i+1], os.Args[i+2:]...)
			return filepath.Clean(val)
		}
	}
	return ""
}

func extractOptionWithValue(optName string) string {
	for i, a := range os.Args[1:] {
		if a == optName && i+2 < len(os.Args) {
			val := os.Args[i+2]
			os.Args = append(os.Args[:i+1], os.Args[i+3:]...)
			return val
		}
		if strings.HasPrefix(a, optName+"=") {
			val := strings.TrimPrefix(a, optName+"=")
			os.Args = append(os.Args[:i+1], os.Args[i+2:]...)
			return val
		}
	}
	return ""
}

func removeFlag(flag string) bool {
	for i, a := range os.Args[1:] {
		if a == flag {
			os.Args = append(os.Args[:i+1], os.Args[i+2:]...)
			return true
		}
	}
	return false
}

func runSelector(triesPath, searchTerm, andType string, andExit bool, andKeys []string, andConfirm string) *selector.Result {
	opts := []selector.Option{
		selector.WithBasePath(triesPath),
	}
	if searchTerm != "" {
		opts = append(opts, selector.WithSearchTerm(searchTerm))
	}
	if andType != "" {
		opts = append(opts, selector.WithAndType(andType))
	}
	if andExit {
		opts = append(opts, selector.WithTestRenderOnce())
		opts = append(opts, selector.WithTestNoCls())
	}
	if andKeys != nil {
		opts = append(opts, selector.WithTestKeys(andKeys))
	}
	if andConfirm != "" {
		opts = append(opts, selector.WithTestConfirm(andConfirm))
	}

	return selector.RunBubbletea(opts...)
}

func isGitWorktree(path string) bool {
	gitPath := filepath.Join(path, ".git")
	info, err := os.Stat(gitPath)
	if err != nil {
		return false
	}
	return !info.IsDir()
}

func handleDotShorthand(pathArg string, rest []string, triesPath string) {
	custom := strings.Join(rest, " ")
	if pathArg == "." && strings.TrimSpace(custom) == "" {
		fmt.Fprintln(os.Stderr, "Error: 'try .' requires a name argument")
		fmt.Fprintln(os.Stderr, "Usage: try . <name>")
		os.Exit(1)
	}

	repoDir, _ := filepath.Abs(pathArg)
	base := ""
	if strings.TrimSpace(custom) != "" {
		base = strings.ReplaceAll(strings.TrimSpace(custom), " ", "-")
	} else {
		base = filepath.Base(repoDir)
	}

	datePrefix := time.Now().Format("2006-01-02")
	base = resolveUniqueName(triesPath, datePrefix, base)
	fullPath := filepath.Join(triesPath, datePrefix+"-"+base)

	gitPath := filepath.Join(repoDir, ".git")
	hasGit := false
	if info, err := os.Stat(gitPath); err == nil {
		hasGit = info.IsDir() || !info.IsDir()
	}

	if hasGit {
		var worktreeRepo string
		if pathArg != "." {
			worktreeRepo = repoDir
		}
		shell.EmitScript(shell.ScriptWorktree(fullPath, worktreeRepo))
	} else {
		shell.EmitScript(shell.ScriptMkdirCd(fullPath))
	}
}

func emitCloneURI(query, triesPath string) {
	parts := strings.SplitN(query, " ", 2)
	uri := parts[0]
	customName := ""
	if len(parts) > 1 {
		customName = parts[1]
	}
	dirName := generateCloneDirName(uri, customName)
	if dirName == "" {
		fmt.Fprintf(os.Stderr, "Error: Unable to parse git URI: %s\n", uri)
		os.Exit(1)
	}
	shell.EmitScript(shell.ScriptClone(filepath.Join(triesPath, dirName), uri))
}

func emitResult(result selector.Result) {
	switch result.Type {
	case selector.ResultDelete:
		shell.EmitScript(shell.ScriptDelete(result.Paths, result.BasePath))
	case selector.ResultMkdir:
		shell.EmitScript(shell.ScriptMkdirCd(result.Path))
	case selector.ResultRename:
		shell.EmitScript(shell.ScriptRename(result.BasePath, result.OldName, result.NewName))
	case selector.ResultAscend:
		isWorktree := isGitWorktree(result.Source)
		shell.EmitScript(shell.ScriptAscend(result.Source, result.Dest, result.Basename, result.BasePath, isWorktree))
	default:
		shell.EmitScript(shell.ScriptCd(result.Path))
	}
}

func cmdClone(args []string, triesPath string) []string {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "Error: git URI required for clone command")
		os.Exit(1)
	}

	uri := args[0]
	customName := ""
	if len(args) > 1 {
		customName = args[1]
	}

	dirName := generateCloneDirName(uri, customName)
	if dirName == "" {
		fmt.Fprintf(os.Stderr, "Error: Unable to parse git URI: %s\n", uri)
		os.Exit(1)
	}

	return shell.ScriptClone(filepath.Join(triesPath, dirName), uri)
}

func cmdWorktree(args []string, triesPath string) []string {
	var repoDir string
	var customName string
	cwd, _ := os.Getwd()

	if len(args) > 0 {
		if args[0] == "dir" {
			repoDir = cwd
			customName = strings.Join(args[1:], " ")
		} else {
			repoDir, _ = filepath.Abs(args[0])
			customName = strings.Join(args[1:], " ")
		}
	}

	return worktreeScript(triesPath, repoDir, customName, cwd)
}

func worktreeScript(triesPath, repoDir, customName, cwd string) []string {
	var worktreeRepo string

	base := ""
	if customName != "" && strings.TrimSpace(customName) != "" {
		base = strings.ReplaceAll(strings.TrimSpace(customName), " ", "-")
	} else {
		if repoDir != "" {
			base = filepath.Base(repoDir)
		} else {
			base = filepath.Base(cwd)
		}
	}

	if repoDir != "" && repoDir != cwd {
		worktreeRepo = repoDir
	}

	datePrefix := time.Now().Format("2006-01-02")
	base = resolveUniqueName(triesPath, datePrefix, base)
	fullPath := filepath.Join(triesPath, datePrefix+"-"+base)

	return shell.ScriptWorktree(fullPath, worktreeRepo)
}

func resolveUniqueName(triesPath, datePrefix, base string) string {
	initial := datePrefix + "-" + base
	if _, err := os.Stat(filepath.Join(triesPath, initial)); os.IsNotExist(err) {
		return base
	}

	re := regexp.MustCompile(`^(.+?)(\d+)$`)
	m := re.FindStringSubmatch(base)
	if m != nil {
		stem, _ := m[1], m[2]
		num := 2
		for {
			candidate := fmt.Sprintf("%s%d", stem, num)
			if _, err := os.Stat(filepath.Join(triesPath, datePrefix+"-"+candidate)); os.IsNotExist(err) {
				return candidate
			}
			num++
		}
	}

	for i := 2; ; i++ {
		candidate := fmt.Sprintf("%s-%d", base, i)
		if _, err := os.Stat(filepath.Join(triesPath, datePrefix+"-"+candidate)); os.IsNotExist(err) {
			return candidate
		}
	}
}

func cmdInit(args []string, triesPath string) {
	scriptPath, _ := os.Executable()

	explicitPath := ""
	if len(args) > 0 && strings.HasPrefix(args[0], "/") {
		explicitPath = args[0]
	}

	defaultPath := triesPath
	if defaultPath == "" {
		if home, err := os.UserHomeDir(); err == nil {
			defaultPath = filepath.Join(home, "src", "tries")
		}
	}

	s := shell.DetectShell()
	fmt.Print(shell.InitSnippet(s, scriptPath, explicitPath, defaultPath))
}

func cmdInstall(args []string, triesPath string) {
	scriptPath, _ := os.Executable()

	explicitPath := ""
	if len(args) > 0 && strings.HasPrefix(args[0], "/") {
		explicitPath = args[0]
	}

	defaultPath := triesPath
	if defaultPath == "" {
		if home, err := os.UserHomeDir(); err == nil {
			defaultPath = filepath.Join(home, "src", "tries")
		}
	}

	shellType := shell.DetectShell()
	rcFile := shellRcFile(shellType)
	if rcFile == "" {
		fmt.Fprintln(os.Stderr, "Error: could not determine shell config file")
		os.Exit(1)
	}

	rcPath, _ := filepath.Abs(expandHome(rcFile))
	if data, err := os.ReadFile(rcPath); err == nil {
		if strings.Contains(string(data), "# try shell integration") {
			fmt.Fprintf(os.Stderr, "try is already installed in %s\n", rcPath)
			os.Exit(0)
		}
	}

	snippet := shell.InitSnippet(shellType, scriptPath, explicitPath, defaultPath)
	block := "\n# try shell integration\n" + snippet

	_ = os.MkdirAll(filepath.Dir(rcPath), 0755)
	f, err := os.OpenFile(rcPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: %s is read-only, skipping.\n", rcPath)
		os.Exit(1)
	}
	defer f.Close()
	f.WriteString(block)

	fmt.Fprintf(os.Stderr, "Added try shell integration to %s\n", rcPath)
}

func shellRcFile(shellType shell.ShellType) string {
	home, _ := os.UserHomeDir()
	switch shellType {
	case shell.ShellFish:
		return filepath.Join(home, ".config", "fish", "config.fish")
	default:
		bashrc := filepath.Join(home, ".bashrc")
		if _, err := os.Stat(bashrc); err == nil {
			return bashrc
		}
		return filepath.Join(home, ".bash_profile")
	}
}

func expandHome(p string) string {
	if strings.HasPrefix(p, "~/") {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, p[2:])
	}
	return p
}

func isGitURI(s string) bool {
	if s == "" {
		return false
	}
	if strings.HasPrefix(s, "https://") || strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "git@") {
		return true
	}
	if strings.HasSuffix(s, ".git") {
		return true
	}
	return false
}

func parseGitURI(uri string) map[string]string {
	uri = strings.TrimSuffix(uri, ".git")

	patterns := []string{
		`^https?://github\.com/([^/]+)/([^/]+)$`,
		`^git@github\.com:([^/]+)/([^/]+)$`,
		`^https?://([^/]+)/([^/]+)/([^/]+)$`,
		`^git@([^:]+):([^/]+)/([^/]+)$`,
	}

	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		m := re.FindStringSubmatch(uri)
		if m == nil {
			continue
		}
		if len(m) == 3 {
			return map[string]string{"user": m[1], "repo": m[2], "host": "github.com"}
		}
		if len(m) == 4 {
			return map[string]string{"host": m[1], "user": m[2], "repo": m[3]}
		}
	}

	return nil
}

func generateCloneDirName(uri, customName string) string {
	if customName != "" {
		return customName
	}
	parsed := parseGitURI(uri)
	if parsed == nil {
		return ""
	}
	datePrefix := time.Now().Format("2006-01-02")
	return fmt.Sprintf("%s-%s-%s", datePrefix, parsed["user"], parsed["repo"])
}

func parseTestKeys(spec string) []string {
	if spec == "" {
		return nil
	}

	if strings.Contains(spec, ",") || regexp.MustCompile(`^[A-Z\-]+$`).MatchString(spec) {
		var keys []string
		tokens := strings.Split(spec, ",")
		for _, tok := range tokens {
			tok = strings.TrimSpace(tok)
			up := strings.ToUpper(tok)
			switch up {
			case "UP":
				keys = append(keys, "\x1b[A")
			case "DOWN":
				keys = append(keys, "\x1b[B")
			case "LEFT":
				keys = append(keys, "\x1b[D")
			case "RIGHT":
				keys = append(keys, "\x1b[C")
			case "ENTER":
				keys = append(keys, "\r")
			case "ESC":
				keys = append(keys, "\x1b")
			case "BACKSPACE":
				keys = append(keys, "\x7f")
			case "CTRL-A", "CTRLA":
				keys = append(keys, "\x01")
			case "CTRL-B", "CTRLB":
				keys = append(keys, "\x02")
			case "CTRL-D", "CTRLD":
				keys = append(keys, "\x04")
			case "CTRL-E", "CTRLE":
				keys = append(keys, "\x05")
			case "CTRL-F", "CTRLF":
				keys = append(keys, "\x06")
			case "CTRL-G", "CTRLG":
				keys = append(keys, "\x07")
			case "CTRL-H", "CTRLH":
				keys = append(keys, "\x08")
			case "CTRL-K", "CTRLK":
				keys = append(keys, "\x0b")
			case "CTRL-N", "CTRLN":
				keys = append(keys, "\x0e")
			case "CTRL-P", "CTRLP":
				keys = append(keys, "\x10")
			case "CTRL-R", "CTRLR":
				keys = append(keys, "\x12")
			case "CTRL-T", "CTRLT":
				keys = append(keys, "\x14")
			case "CTRL-W", "CTRLW":
				keys = append(keys, "\x17")
			default:
				if strings.HasPrefix(strings.ToUpper(tok), "TYPE=") {
					for _, ch := range tok[5:] {
						keys = append(keys, string(ch))
					}
				} else if len(tok) == 1 {
					keys = append(keys, tok)
				}
			}
		}
		return keys
	}

	// Raw character mode
	var keys []string
	runes := []rune(spec)
	i := 0
	for i < len(runes) {
		if runes[i] == '\x1b' && i+2 < len(runes) && runes[i+1] == '[' {
			keys = append(keys, string(runes[i:i+3]))
			i += 3
		} else {
			keys = append(keys, string(runes[i]))
			i++
		}
	}
	return keys
}

func printHelp() {
	fmt.Fprintf(os.Stderr, `try v%s - ephemeral workspace manager

To use try, add to your shell config:

  # bash/zsh (~/.bashrc or ~/.zshrc)
  eval "$(try init ~/src/tries)"

  # fish (~/.config/fish/config.fish)
  eval (try init ~/src/tries | string collect)

Usage:
  try [query]           Interactive directory selector
  try clone <url>       Clone repo into dated directory
  try worktree <name>   Create worktree from current git repo
  try --help            Show this help

Commands:
  init [path]           Output shell function definition
  clone <url> [name]    Clone git repo into date-prefixed directory
  worktree <name>       Create worktree in dated directory

Environment:
  TRY_PATH          Tries directory (default: ~/src/tries)
  TRY_PROJECTS      Graduate destination (default: parent of TRY_PATH)

Keyboard:
  ↑/↓, Ctrl-P/N     Navigate
  Enter              Select / Create new
  Ctrl-R             Rename
  Ctrl-G             Graduate (promote try to project)
  Ctrl-D             Mark for deletion
  Ctrl-T             Create new try
  Esc                Cancel
`, version)
}
