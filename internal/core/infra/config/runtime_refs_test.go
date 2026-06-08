package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	base "getytstatsapi/pkg/config"
)

func TestLoadAndSaveRuntimeRefs(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.json")
	chatUsersPath := filepath.Join(dir, "chat_users.json")
	securityPath := filepath.Join(dir, ".security.yml")

	configData := `{
  "logger_level": "debug",
  "http": {"address": ":80", "base_url": "https://example.com"},
  "database": {"host": "127.0.0.1", "port": 5432, "user": "test", "password": "test", "name": "test"},
  "features": {
    "stintinside": {"youtube_api_key": "test-youtube-key"}
  },
  "chat_users": "file://chat_users.json"
}`
	chatUsersData := `{"123":{"456":{"id":456,"first_name":"Test"}}}`

	if err := os.WriteFile(configPath, []byte(configData), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(chatUsersPath, []byte(chatUsersData), 0o600); err != nil {
		t.Fatal(err)
	}

	loader, err := base.NewLoader(
		base.WithConfigPath(configPath),
		base.WithSecurityPath(securityPath),
		base.WithEnvironment(false),
	)
	if err != nil {
		t.Fatal(err)
	}
	cfg := DefaultConfig()
	if err := loader.LoadInto(cfg); err != nil {
		t.Fatal(err)
	}
	if err := LoadRuntimeRefs(cfg, loader.ConfigPath()); err != nil {
		t.Fatal(err)
	}
	if got := cfg.ChatUsers[123][456].FirstName; got != "Test" {
		t.Fatalf("chat user = %q, want Test", got)
	}

	cfg.ChatUsers[123][789] = KnownUser{ID: 789, FirstName: "Saved"}
	if err := SaveRuntimeRefs(cfg, loader.ConfigPath()); err != nil {
		t.Fatal(err)
	}
	if err := loader.Save(cfg); err != nil {
		t.Fatal(err)
	}

	var savedConfig map[string]any
	readJSON(t, configPath, &savedConfig)
	if savedConfig["chat_users"] != "file://chat_users.json" {
		t.Fatalf("chat_users ref = %#v", savedConfig["chat_users"])
	}

	var savedChatUsers ChatUsersConfig
	readJSON(t, chatUsersPath, &savedChatUsers)
	if got := savedChatUsers[123][789].FirstName; got != "Saved" {
		t.Fatalf("saved chat user = %q, want Saved", got)
	}
}

func readJSON(t *testing.T, path string, target any) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, target); err != nil {
		t.Fatal(err)
	}
}
