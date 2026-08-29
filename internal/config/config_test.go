package config

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDefaults(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "default_test.txt")

	if err := defaultWriteFile(filePath, []byte("data"), 0o600); err != nil {
		t.Fatalf("defaultWriteFile error: %v", err)
	}
	if err := defaultWriteFile(filepath.Join(tempDir, "missing", "file.txt"), []byte("data"), 0o600); err == nil {
		t.Fatal("expected defaultWriteFile error")
	}

	data, err := defaultReadFile(filePath)
	if err != nil || string(data) != "data" {
		t.Fatalf("defaultReadFile error = %v, got %s", err, string(data))
	}
	if _, readErr := defaultReadFile(filepath.Join(tempDir, "non_existent.txt")); readErr == nil {
		t.Fatal("expected defaultReadFile error")
	}

	subDir := filepath.Join(tempDir, "sub", "dir")
	if mkdirErr := defaultMkdirAll(subDir, 0o700); mkdirErr != nil {
		t.Fatalf("defaultMkdirAll error: %v", mkdirErr)
	}
	if mkdirErr := defaultMkdirAll(filePath, 0o700); mkdirErr == nil {
		t.Fatal("expected defaultMkdirAll error")
	}

	indentData, err := defaultJSONMarshalIndent(map[string]string{"k": "v"}, "", "  ")
	if err != nil || !strings.Contains(string(indentData), "k") {
		t.Fatalf("defaultJSONMarshalIndent error = %v", err)
	}

	t.Setenv("HOME", "")
	t.Setenv("AppData", "")
	t.Setenv("XDG_CONFIG_HOME", "")
	if _, errEmpty := defaultUserConfigDir(); errEmpty == nil {
		t.Log("config dir empty")
	}

	t.Setenv("HOME", tempDir)
	t.Setenv("AppData", tempDir)
	t.Setenv("XDG_CONFIG_HOME", tempDir)
	dir, err := defaultUserConfigDir()
	if err != nil || dir == "" {
		t.Fatalf("defaultUserConfigDir error = %v, dir = %s", err, dir)
	}
}

func TestLoad(t *testing.T) {
	tempDir := t.TempDir()

	origUserConfigDir := userConfigDir
	origReadFile := readFile
	defer func() {
		userConfigDir = origUserConfigDir
		readFile = origReadFile
	}()

	t.Run("config_path_error", func(t *testing.T) {
		userConfigDir = func() (string, error) {
			return "", errors.New("mock config dir error")
		}
		_, err := Load()
		if err == nil || !strings.Contains(err.Error(), "mock config dir error") {
			t.Fatalf("expected error, got %v", err)
		}
	})

	t.Run("file_not_exist", func(t *testing.T) {
		userConfigDir = func() (string, error) {
			return tempDir, nil
		}
		cfg, err := Load()
		if err != nil {
			t.Fatalf("expected nil error, got %v", err)
		}
		if cfg.OutputDir != "" {
			t.Fatalf("expected empty OutputDir, got %s", cfg.OutputDir)
		}
	})

	t.Run("read_file_error", func(t *testing.T) {
		userConfigDir = func() (string, error) {
			return tempDir, nil
		}
		readFile = func(_ string) ([]byte, error) {
			return nil, errors.New("mock read error")
		}
		_, err := Load()
		if err == nil || !strings.Contains(err.Error(), "mock read error") {
			t.Fatalf("expected read error, got %v", err)
		}
	})

	t.Run("parse_json_error", func(t *testing.T) {
		userConfigDir = func() (string, error) {
			return tempDir, nil
		}
		readFile = func(_ string) ([]byte, error) {
			return []byte("invalid-json"), nil
		}
		_, err := Load()
		if err == nil || !strings.Contains(err.Error(), "parse config file") {
			t.Fatalf("expected parse error, got %v", err)
		}
	})

	t.Run("load_success", func(t *testing.T) {
		userConfigDir = func() (string, error) {
			return tempDir, nil
		}
		readFile = func(_ string) ([]byte, error) {
			return []byte(`{"output_dir":"/custom/path"}`), nil
		}
		cfg, err := Load()
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if cfg.OutputDir != "/custom/path" {
			t.Fatalf("expected /custom/path, got %s", cfg.OutputDir)
		}
	})
}

func TestSave(t *testing.T) {
	tempDir := t.TempDir()

	origUserConfigDir := userConfigDir
	origMkdirAll := mkdirAll
	origMarshal := jsonMarshalIndent
	origWriteFile := writeFile
	defer func() {
		userConfigDir = origUserConfigDir
		mkdirAll = origMkdirAll
		jsonMarshalIndent = origMarshal
		writeFile = origWriteFile
	}()

	t.Run("config_path_error", func(t *testing.T) {
		userConfigDir = func() (string, error) {
			return "", errors.New("mock config dir error")
		}
		err := Save(&Config{OutputDir: "test-err-path-1"})
		if err == nil || !strings.Contains(err.Error(), "mock config dir error") {
			t.Fatalf("expected error, got %v", err)
		}
	})

	t.Run("mkdir_error", func(t *testing.T) {
		userConfigDir = func() (string, error) {
			return tempDir, nil
		}
		mkdirAll = func(_ string, _ os.FileMode) error {
			return errors.New("mock mkdir error")
		}
		err := Save(&Config{OutputDir: "test-err-path-2"})
		if err == nil || !strings.Contains(err.Error(), "mock mkdir error") {
			t.Fatalf("expected mkdir error, got %v", err)
		}
	})

	t.Run("marshal_error", func(t *testing.T) {
		userConfigDir = func() (string, error) {
			return tempDir, nil
		}
		mkdirAll = origMkdirAll
		jsonMarshalIndent = func(_ any, _, _ string) ([]byte, error) {
			return nil, errors.New("mock marshal error")
		}
		err := Save(&Config{OutputDir: "test-err-path-3"})
		if err == nil || !strings.Contains(err.Error(), "mock marshal error") {
			t.Fatalf("expected marshal error, got %v", err)
		}
	})

	t.Run("write_error", func(t *testing.T) {
		userConfigDir = func() (string, error) {
			return tempDir, nil
		}
		mkdirAll = origMkdirAll
		jsonMarshalIndent = origMarshal
		writeFile = func(_ string, _ []byte, _ os.FileMode) error {
			return errors.New("mock write error")
		}
		err := Save(&Config{OutputDir: "test-err-path-4"})
		if err == nil || !strings.Contains(err.Error(), "mock write error") {
			t.Fatalf("expected write error, got %v", err)
		}
	})

	t.Run("save_success", func(t *testing.T) {
		userConfigDir = func() (string, error) {
			return tempDir, nil
		}
		mkdirAll = origMkdirAll
		jsonMarshalIndent = origMarshal
		writeFile = origWriteFile

		err := Save(&Config{OutputDir: "/saved/path"})
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		cfg, err := Load()
		if err != nil || cfg.OutputDir != "/saved/path" {
			t.Fatalf("expected loaded OutputDir /saved/path, got %v, err %v", cfg, err)
		}
	})
}

func TestGetSetClearOutputDir(t *testing.T) {
	tempDir := t.TempDir()

	origUserConfigDir := userConfigDir
	defer func() {
		userConfigDir = origUserConfigDir
	}()

	t.Run("get_output_dir_load_error", func(t *testing.T) {
		userConfigDir = func() (string, error) {
			return "", errors.New("mock error")
		}
		if got := GetOutputDir(); got != "" {
			t.Fatalf("expected empty string on load error, got %s", got)
		}
	})

	t.Run("set_and_get_and_clear_success", func(t *testing.T) {
		userConfigDir = func() (string, error) {
			return tempDir, nil
		}

		if got := GetOutputDir(); got != "" {
			t.Fatalf("expected initial empty string, got %s", got)
		}

		if err := SetOutputDir("/my/books/"); err != nil {
			t.Fatalf("SetOutputDir error: %v", err)
		}

		if got := GetOutputDir(); got != filepath.Clean("/my/books/") {
			t.Fatalf("expected cleaned path, got %s", got)
		}

		if err := ClearOutputDir(); err != nil {
			t.Fatalf("ClearOutputDir error: %v", err)
		}

		if got := GetOutputDir(); got != "" {
			t.Fatalf("expected empty string after clear, got %s", got)
		}
	})

	t.Run("set_and_clear_load_error", func(t *testing.T) {
		userConfigDir = func() (string, error) {
			return "", errors.New("mock error")
		}
		if err := SetOutputDir("/path"); err == nil {
			t.Fatal("expected error on SetOutputDir when load fails")
		}
		if err := ClearOutputDir(); err == nil {
			t.Fatal("expected error on ClearOutputDir when load fails")
		}
	})
}
