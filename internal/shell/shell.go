package shell

import (
	"fmt"
	"strings"
)

const Warning = "# if you can read this, you didn't launch try from an alias. run try --help."

func Q(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'"
}

func EmitScript(cmds []string) {
	fmt.Println(Warning)
	for i, cmd := range cmds {
		if i == 0 {
			fmt.Print(cmd)
		} else {
			fmt.Print("  " + cmd)
		}
		if i < len(cmds)-1 {
			fmt.Println(" && \\")
		} else {
			fmt.Println()
		}
	}
}

func ScriptCd(path string) []string {
	return []string{
		"touch " + Q(path),
		"echo " + Q(path),
		"cd " + Q(path),
	}
}

func ScriptMkdirCd(path string) []string {
	return append([]string{"mkdir -p " + Q(path)}, ScriptCd(path)...)
}

func ScriptClone(path, uri string) []string {
	return append([]string{
		"mkdir -p " + Q(path),
		"echo " + Q("Using git clone to create this trial from "+uri+"."),
		"git clone " + Q(uri) + " " + Q(path),
	}, ScriptCd(path)...)
}

func ScriptWorktree(path, repo string) []string {
	var worktreeCmd string
	if repo != "" {
		worktreeCmd = "/usr/bin/env sh -c 'if git -C " + Q(repo) + " rev-parse --is-inside-work-tree >/dev/null 2>&1; then repo=$(git -C " + Q(repo) + " rev-parse --show-toplevel); git -C \"$repo\" worktree add --detach " + Q(path) + " >/dev/null 2>&1 || true; fi; exit 0'"
	} else {
		worktreeCmd = "/usr/bin/env sh -c 'if git rev-parse --is-inside-work-tree >/dev/null 2>&1; then repo=$(git rev-parse --show-toplevel); git -C \"$repo\" worktree add --detach " + Q(path) + " >/dev/null 2>&1 || true; fi; exit 0'"
	}
	src := repo
	if src == "" {
		src = "."
	}
	return append([]string{
		"mkdir -p " + Q(path),
		"echo " + Q("Using git worktree to create this trial from "+src+"."),
		worktreeCmd,
	}, ScriptCd(path)...)
}

func ScriptDelete(paths []string, basePath string) []string {
	cmds := []string{"cd " + Q(basePath)}
	for _, name := range paths {
		cmds = append(cmds, "test -d "+Q(name)+" && rm -rf "+Q(name))
	}
	cmds = append(cmds, "cd "+Q(".")+" 2>/dev/null || cd "+Q(basePath))
	return cmds
}

func ScriptRename(basePath, oldName, newName string) []string {
	newPath := basePath + "/" + newName
	return []string{
		"cd " + Q(basePath),
		"mv " + Q(oldName) + " " + Q(newName),
		"echo " + Q(newPath),
		"cd " + Q(newPath),
	}
}

func ScriptAscend(source, dest, basename, basePath string, isWorktree bool) []string {
	symlinkPath := basePath + "/" + basename
	cmds := []string{}
	if isWorktree {
		cmds = append(cmds, "git worktree move "+Q(source)+" "+Q(dest))
	} else {
		cmds = append(cmds, "mv "+Q(source)+" "+Q(dest))
	}
	cmds = append(cmds, "ln -s "+Q(dest)+" "+Q(symlinkPath))
	cmds = append(cmds, "echo "+Q("Graduated: "+basename+" -> "+dest))
	return append(cmds, ScriptCd(dest)...)
}
