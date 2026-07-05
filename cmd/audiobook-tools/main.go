// Package main provides the audiobook-tools command line interface.
package main

import (
	"fmt"
	"io"
	"os"

	"github.com/underhax/audiobook-tools/internal/cli"
)

var osExit = os.Exit

func main() {
	osExit(run(os.Args, os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) < 2 {
		printUsage(stdout)
		return 1
	}

	command := args[1]
	cmdArgs := args[2:]

	var err error
	switch command {
	case "download":
		err = cli.RunDownload(cmdArgs, stdout)
	case "auth":
		err = cli.RunAuth(cmdArgs, stdout)
	case "build":
		err = cli.RunBuild(cmdArgs, stdout)
	case "help", "-h", "--help":
		printUsage(stdout)
		return 0
	case "version", "-v", "--version":
		if _, fmtErr := fmt.Fprintf(stdout, "audiobook-tools version %s\n", cli.AppVersion); fmtErr != nil {
			return 1
		}
		return 0
	default:
		if _, fmtErr := fmt.Fprintf(stderr, "Unknown command: %s\n", command); fmtErr != nil {
			return 1
		}
		printUsage(stderr)
		return 1
	}

	if err != nil {
		if _, printErr := fmt.Fprintf(stderr, "Error: %v\n", err); printErr != nil {
			return 1
		}
		return 1
	}

	return 0
}

func printUsage(out io.Writer) {
	if _, err := fmt.Fprintln(out, `audiobook-tools - A suite of utilities for downloading and building audiobooks.

Usage:
  audiobook-tools <command> [arguments]

The commands are:
  auth        Save authentication token for a specific provider (e.g. books_yandex)
  download    Download an audiobook from a supported site
  build       Build an M4B file from an existing directory of MP3s
  version     Print the version number

Use "audiobook-tools <command> -h" for more information about a command.`); err != nil {
		return
	}
}
