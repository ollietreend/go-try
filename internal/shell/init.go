package shell

import (
	"fmt"
	"os"
	"strings"
)

type ShellType int

const (
	ShellBash ShellType = iota
	ShellZsh
	ShellFish
)

func DetectShell() ShellType {
	shell := os.Getenv("SHELL")
	if strings.Contains(shell, "fish") {
		return ShellFish
	}
	return ShellBash // bash and zsh use the same init format
}

func InitSnippet(shell ShellType, scriptPath, explicitPath, defaultPath string) string {
	switch shell {
	case ShellFish:
		return initFish(scriptPath, explicitPath, defaultPath)
	default:
		return initBash(scriptPath, explicitPath, defaultPath)
	}
}

func initBash(scriptPath, explicitPath, defaultPath string) string {
	pathArg := ""
	if explicitPath != "" {
		pathArg = fmt.Sprintf(" --path '%s'", explicitPath)
	} else {
		pathArg = fmt.Sprintf(" --path \"${TRY_PATH:-%s}\"", defaultPath)
	}
	return fmt.Sprintf(
		`try() {
  local out
  out=$(/usr/bin/env %s exec%s "$@" 2>/dev/tty)
  if [ $? -eq 0 ]; then
    eval "$out"
  else
    echo "$out"
  fi
}
`, scriptPath, pathArg)
}

func initFish(scriptPath, explicitPath, defaultPath string) string {
	pathArg := ""
	if explicitPath != "" {
		pathArg = fmt.Sprintf(" --path '%s'", explicitPath)
	} else {
		pathArg = fmt.Sprintf(" --path (if set -q TRY_PATH; echo \"$TRY_PATH\"; else; echo '%s'; end)", defaultPath)
	}
	return fmt.Sprintf(
		`function try
  set -l out (/usr/bin/env %s exec%s $argv 2>/dev/tty | string collect)
  if test $pipestatus[1] -eq 0
    eval $out
  else
    echo $out
  end
end
`, scriptPath, pathArg)
}
