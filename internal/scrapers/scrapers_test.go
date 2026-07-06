package scrapers

import (
	"testing"
)

func TestDefaultUserConfigDir(t *testing.T) {
	t.Setenv("HOME", "")
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("APPDATA", "")
	t.Setenv("USERPROFILE", "")

	_, err := defaultUserConfigDir()
	if err == nil {
		t.Fatal("expected error from defaultUserConfigDir when env vars are empty")
	}

	t.Setenv("HOME", t.TempDir())
	dir, err := defaultUserConfigDir()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dir == "" {
		t.Fatal("expected non-empty dir")
	}
}

func TestDefaultJSONMarshalIndent(t *testing.T) {
	_, err := defaultJSONMarshalIndent(map[string]string{"test": "test"}, "", "  ")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
