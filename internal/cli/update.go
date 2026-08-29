package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"

	"github.com/underhax/audiobook-tools/internal/updater"
)

func defaultUpdaterUpdate(ctx context.Context, client *http.Client, currentVersion string) error {
	if err := updater.Update(ctx, client, currentVersion); err != nil {
		return fmt.Errorf("updater update: %w", err)
	}
	return nil
}

var (
	updaterUpdate       = defaultUpdaterUpdate
	defaultUpdateClient = http.DefaultClient
)

// RunUpdate handles flag parsing and delegates execution to the binary updater using the current application release version.
func RunUpdate(args []string, out io.Writer) error {
	fs := flag.NewFlagSet("update", flag.ContinueOnError)
	fs.SetOutput(out)

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return fmt.Errorf("parse flags: %w", err)
	}

	if len(fs.Args()) > 0 {
		return fmt.Errorf("unexpected argument: %s", fs.Args()[0])
	}

	if err := updaterUpdate(context.Background(), defaultUpdateClient, AppVersion); err != nil {
		return fmt.Errorf("update failed: %w", err)
	}

	return nil
}
