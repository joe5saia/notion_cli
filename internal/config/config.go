// Package config manages disk and keyring state for notionctl profiles.
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"
	"github.com/zalando/go-keyring"
)

const (
	serviceName          = "notionctl"
	defaultNotionVersion = "2025-09-03"

	dirPermissions  = 0o700
	filePermissions = 0o600
)

// DefaultNotionVersion exposes the API version we pin to unless the user overrides it.
func DefaultNotionVersion() string {
	return defaultNotionVersion
}

// configDir returns the directory where we persist structured configuration.
func configDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home directory: %w", err)
	}
	return filepath.Join(home, ".config", "notionctl"), nil
}

// ensureConfigDir ensures the configuration directory exists with restricted permissions.
func ensureConfigDir() (string, error) {
	dir, err := configDir()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, dirPermissions); err != nil {
		return "", fmt.Errorf("create config directory: %w", err)
	}
	return dir, nil
}

// SaveToken stores the integration token for the provided profile in the OS keyring.
// It also records the Notion API version alongside the credential metadata.
func SaveToken(profile, token, version string) error {
	token = strings.TrimSpace(token)
	if token == "" {
		return errors.New("token cannot be empty")
	}
	if profile == "" {
		return errors.New("profile name cannot be empty")
	}
	if version == "" {
		version = defaultNotionVersion
	}

	if err := keyring.Set(serviceName, profile, token); err != nil {
		return fmt.Errorf("save token: %w", err)
	}
	if err := SaveVersion(profile, version); err != nil {
		return err
	}
	return nil
}

// SaveVersion persists the target Notion API version for a profile.
func SaveVersion(profile, version string) error {
	if profile == "" {
		return errors.New("profile name cannot be empty")
	}
	if version == "" {
		version = defaultNotionVersion
	}

	dir, err := ensureConfigDir()
	if err != nil {
		return err
	}

	cfg := viper.New()
	configPath := filepath.Join(dir, "config.yaml")
	cfg.SetConfigFile(configPath)
	readErr := cfg.ReadInConfig()
	if readErr != nil && !isConfigNotFound(readErr) {
		return fmt.Errorf("read config: %w", readErr)
	}

	key := fmt.Sprintf("profiles.%s.notion_version", profile)
	cfg.Set(key, version)

	if err := cfg.WriteConfigAs(configPath); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	if err := os.Chmod(configPath, filePermissions); err != nil {
		return fmt.Errorf("restrict config permissions: %w", err)
	}
	return nil
}

// LoadAuth returns the stored token and Notion API version for a profile.
func LoadAuth(profile string) (token, notionVersion string, err error) {
	if profile == "" {
		return "", "", errors.New("profile name cannot be empty")
	}

	tok, err := keyring.Get(serviceName, profile)
	if err != nil {
		if errors.Is(err, keyring.ErrNotFound) {
			return "", "", fmt.Errorf("load token: no stored credentials for profile %q", profile)
		}
		return "", "", fmt.Errorf("load token: %w", err)
	}

	ver, err := LoadVersion(profile)
	if err != nil {
		return "", "", err
	}
	return tok, ver, nil
}

// LoadVersion fetches the configured Notion API version for a profile, falling back to the default.
func LoadVersion(profile string) (string, error) {
	if profile == "" {
		return "", errors.New("profile name cannot be empty")
	}

	dir, err := ensureConfigDir()
	if err != nil {
		return "", err
	}

	cfg := viper.New()
	configPath := filepath.Join(dir, "config.yaml")
	cfg.SetConfigFile(configPath)
	readErr := cfg.ReadInConfig()
	if readErr != nil {
		if isConfigNotFound(readErr) {
			return defaultNotionVersion, nil
		}
		return "", fmt.Errorf("read config: %w", readErr)
	}

	key := fmt.Sprintf("profiles.%s.notion_version", profile)
	ver := cfg.GetString(key)
	if ver == "" {
		return defaultNotionVersion, nil
	}
	return ver, nil
}

func isConfigNotFound(err error) bool {
	if err == nil {
		return false
	}
	var nf viper.ConfigFileNotFoundError
	if errors.As(err, &nf) {
		return true
	}
	var pathErr *os.PathError
	if errors.As(err, &pathErr) && errors.Is(pathErr.Err, os.ErrNotExist) {
		return true
	}
	return errors.Is(err, os.ErrNotExist)
}
