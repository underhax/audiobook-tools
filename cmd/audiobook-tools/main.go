// Package main provides the audiobook-tools command line interface.
package main

import (
	"fmt"
	"os"

	"github.com/underhax/audiobook-tools/internal/cli"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	command := os.Args[1]
	args := os.Args[2:]

	var err error
	switch command {
	case "download":
		err = cli.RunDownload(args, os.Stdout)
	case "build":
		err = cli.RunBuild(args, os.Stdout)
	case "help", "-h", "--help":
		printUsage()
		return
	case "version", "-v", "--version":
		fmt.Printf("audiobook-tools version %s\n", cli.AppVersion)
		return
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", command)
		printUsage()
		os.Exit(1)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println(`audiobook-tools - A suite of utilities for downloading and building audiobooks.

Usage:
  audiobook-tools <command> [arguments]

The commands are:
  download    Download an audiobook from a supported site
  build       Build an M4B file from an existing directory of MP3s
  version     Print the version number

Use "audiobook-tools <command> -h" for more information about a command.`)
}
