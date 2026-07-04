package cli

import (
	"errors"
	"flag"
	"fmt"
	"io"

	"github.com/underhax/audiobook-tools/internal/scrapers"
)

// RunAuth parses flags and saves authentication tokens for specific providers.
func RunAuth(args []string, out io.Writer) error {
	fs := flag.NewFlagSet("auth", flag.ContinueOnError)
	fs.SetOutput(out)

	fs.Usage = func() {
		_, err := fmt.Fprint(out, "Usage: auth <provider> <token>\n\nProviders:\n  books_yandex\n")
		_ = err
	}

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return fmt.Errorf("parse flags: %w", err)
	}

	positionalArgs := fs.Args()
	if len(positionalArgs) == 0 {
		return errors.New("provider name is required (e.g. 'audiobook-tools auth books_yandex <token>')")
	}
	provider := positionalArgs[0]

	if len(positionalArgs) < 2 {
		return errors.New("token is required (e.g. 'audiobook-tools auth books_yandex <token>')")
	}
	token := positionalArgs[1]

	switch provider {
	case "books_yandex":
		path, err := scrapers.SaveToken(token)
		if err != nil {
			return fmt.Errorf("save books_yandex token: %w", err)
		}
		if _, err := fmt.Fprintf(out, "Successfully saved token for %s to %s\n", provider, path); err != nil {
			return fmt.Errorf("write output: %w", err)
		}
	default:
		return fmt.Errorf("unsupported provider: %s", provider)
	}

	return nil
}
