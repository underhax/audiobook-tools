package cli

import (
	"errors"
	"flag"
	"fmt"
	"io"

	"github.com/underhax/audiobook-tools/internal/config"
)

func defaultConfigLoad() (*config.Config, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, fmt.Errorf("config load: %w", err)
	}
	return cfg, nil
}

func defaultConfigSetOutputDir(dir string) error {
	if err := config.SetOutputDir(dir); err != nil {
		return fmt.Errorf("config set output dir: %w", err)
	}
	return nil
}

func defaultConfigClearOutputDir() error {
	if err := config.ClearOutputDir(); err != nil {
		return fmt.Errorf("config clear output dir: %w", err)
	}
	return nil
}

var (
	configLoad           = defaultConfigLoad
	configSetOutputDir   = defaultConfigSetOutputDir
	configClearOutputDir = defaultConfigClearOutputDir
)

// RunConfig handles viewing and updating application configuration settings.
func RunConfig(args []string, out io.Writer) error {
	fs := flag.NewFlagSet("config", flag.ContinueOnError)
	fs.SetOutput(out)
	setupUsage(fs, "config")

	outDir := fs.String("out", "", "Set default output directory for downloaded audiobooks")
	clearOut := fs.Bool("clear-out", false, "Clear configured default output directory")

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return fmt.Errorf("parse flags: %w", err)
	}

	if len(fs.Args()) > 0 {
		return fmt.Errorf("unexpected argument: %s", fs.Args()[0])
	}

	if *outDir != "" && *clearOut {
		return errors.New("cannot specify both --out and --clear-out")
	}

	if *outDir != "" {
		if err := configSetOutputDir(*outDir); err != nil {
			return fmt.Errorf("set output directory: %w", err)
		}
		if _, err := fmt.Fprintf(out, "Default output directory set to: %s\n", *outDir); err != nil {
			return fmt.Errorf("write output: %w", err)
		}
		return nil
	}

	if *clearOut {
		if err := configClearOutputDir(); err != nil {
			return fmt.Errorf("clear output directory: %w", err)
		}
		if _, err := fmt.Fprintln(out, "Default output directory cleared."); err != nil {
			return fmt.Errorf("write output: %w", err)
		}
		return nil
	}

	cfg, err := configLoad()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	dir := cfg.OutputDir
	if dir == "" {
		dir = "(not set)"
	}

	if _, err := fmt.Fprintf(out, "Default output directory: %s\n", dir); err != nil {
		return fmt.Errorf("write output: %w", err)
	}

	return nil
}
