package config_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zalando/go-keyring"

	"github.com/yourorg/notionctl/internal/config"
)

func TestSaveAndLoadToken(t *testing.T) {
	home := setupHome(t)
	keyring.MockInit()

	const (
		profile = "default"
		token   = "secret_test_token"
		version = "2025-10-01"
	)

	if err := config.SaveToken(profile, token, version); err != nil {
		t.Fatalf("SaveToken returned error: %v", err)
	}

	gotToken, gotVersion, err := config.LoadAuth(profile)
	if err != nil {
		t.Fatalf("LoadAuth returned error: %v", err)
	}
	if gotToken != token {
		t.Fatalf("LoadAuth token = %q, want %q", gotToken, token)
	}
	if gotVersion != version {
		t.Fatalf("LoadAuth version = %q, want %q", gotVersion, version)
	}

	configPath := filepath.Join(home, ".config", "notionctl", "config.yaml")
	info, err := os.Stat(configPath)
	if err != nil {
		t.Fatalf("stat config file: %v", err)
	}
	if mode := info.Mode().Perm(); mode != 0o600 {
		t.Fatalf("config file permissions = %o, want 600", mode)
	}
}

func TestLoadVersionDefault(t *testing.T) {
	setupHome(t)
	keyring.MockInit()

	got, err := config.LoadVersion("default")
	if err != nil {
		t.Fatalf("LoadVersion returned error: %v", err)
	}
	if want := config.DefaultNotionVersion(); got != want {
		t.Fatalf("LoadVersion = %q, want %q", got, want)
	}
}

func TestSaveTokenValidation(t *testing.T) {
	setupHome(t)
	keyring.MockInit()

	if err := config.SaveToken("", "token", ""); err == nil {
		t.Fatalf("SaveToken with empty profile expected error")
	}
	if err := config.SaveToken("default", "   ", ""); err == nil {
		t.Fatalf("SaveToken with empty token expected error")
	}
}

func setupHome(t *testing.T) string {
	t.Helper()

	base := filepath.Join("testdata", "tmp")
	if err := os.MkdirAll(base, 0o750); err != nil {
		t.Fatalf("create base tmp dir: %v", err)
	}

	name := strings.ReplaceAll(t.Name(), "/", "_")
	home := filepath.Join(base, name)
	if err := os.MkdirAll(home, 0o750); err != nil {
		t.Fatalf("create home dir: %v", err)
	}

	t.Cleanup(func() {
		if err := os.RemoveAll(home); err != nil && !os.IsNotExist(err) {
			t.Fatalf("cleanup remove home: %v", err)
		}
		entries, err := os.ReadDir(base)
		if err == nil && len(entries) == 0 {
			if err := os.Remove(base); err != nil && !os.IsNotExist(err) {
				t.Fatalf("cleanup remove base: %v", err)
			}
		}
	})

	t.Setenv("HOME", home)
	return home
}
