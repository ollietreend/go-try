package main

import (
	"fmt"
	"os"
)

const version = "0.1.0"

func main() {
	args := os.Args[1:]

	for _, a := range args {
		if a == "--version" || a == "-v" {
			fmt.Fprintf(os.Stderr, "try %s\n", version)
			os.Exit(0)
		}
	}

	for _, a := range args {
		if a == "--help" || a == "-h" {
			printHelp()
			os.Exit(0)
		}
	}

	printHelp()
}

func printHelp() {
	fmt.Fprint(os.Stderr, `try v`+version+` - ephemeral workspace manager

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
`)
}
