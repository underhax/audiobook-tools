// Package config manages persistent application configuration.
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

func defaultUserConfigDir() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("user config dir: %w", err)
	}
	return dir, nil
}

func defaultReadFile(name string) ([]byte, error) {
	data, err := os.ReadFile(filepath.Clean(name))
	if err != nil {
		return nil, fmt.Errorf("read file: %w", err)
	}
	return data, nil
}

func defaultWriteFile(name string, data []byte, perm os.FileMode) error {
	if err := os.WriteFile(name, data, perm); err != nil {
		return fmt.Errorf("write file: %w", err)
	}
	return nil
}

func defaultMkdirAll(path string, perm os.FileMode) error {
	if err := os.MkdirAll(path, perm); err != nil {
		return fmt.Errorf("mkdir all: %w", err)
	}
	return nil
}

func defaultJSONMarshalIndent(v any, prefix, indent string) ([]byte, error) {
	return json.MarshalIndent(v, prefix, indent)
}

var (
	userConfigDir     = defaultUserConfigDir
	readFile          = defaultReadFile
	writeFile         = defaultWriteFile
	mkdirAll          = defaultMkdirAll
	jsonMarshalIndent = defaultJSONMarshalIndent
)

const (
	appConfigDirName = "audiobook-tools"
	configFileName   = "config.json"
	dirPermissions   = 0o700
	filePermissions  = 0o600
)

// Config represents persistent application settings.
type Config struct {
	OutputDir string `json:"output_dir,omitempty"`
}

func getConfigPath() (string, error) {
	baseDir, err := userConfigDir()
	if err != nil {
		return "", fmt.Errorf("get user config dir: %w", err)
	}
	return filepath.Join(baseDir, appConfigDirName, configFileName), nil
}

// Load reads the application configuration file from user config directory.
func Load() (*Config, error) {
	configPath, err := getConfigPath()
	if err != nil {
		return nil, err
	}

	data, err := readFile(configPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return &Config{}, nil
		}
		return nil, fmt.Errorf("read config file: %w", err)
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config file: %w", err)
	}

	return &cfg, nil
}

// Save writes the given configuration to user config directory.
func Save(cfg *Config) error {
	configPath, err := getConfigPath()
	if err != nil {
		return err
	}

	appDir := filepath.Dir(configPath)
	if mkdirErr := mkdirAll(appDir, dirPermissions); mkdirErr != nil {
		return fmt.Errorf("create config dir: %w", mkdirErr)
	}

	data, err := jsonMarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	if err := writeFile(configPath, data, filePermissions); err != nil {
		return fmt.Errorf("write config file: %w", err)
	}

	return nil
}

// GetOutputDir returns the configured default output directory, or empty string if unset.
func GetOutputDir() string {
	cfg, err := Load()
	if err != nil || cfg == nil {
		return ""
	}
	return cfg.OutputDir
}

// SetOutputDir sets and persists the default output directory.
func SetOutputDir(dir string) error {
	cfg, err := Load()
	if err != nil {
		return err
	}
	cfg.OutputDir = filepath.Clean(dir)
	return Save(cfg)
}

// ClearOutputDir removes the configured default output directory.
func ClearOutputDir() error {
	cfg, err := Load()
	if err != nil {
		return err
	}
	cfg.OutputDir = ""
	return Save(cfg)
}
