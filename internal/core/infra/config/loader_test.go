package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	base "github.com/ra1phdd/config"
)

func TestLoadMergesConfigAndSecurityFiles(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.json")
	securityPath := filepath.Join(dir, ".security.yml")

	configData := `{
		"logger_level": "debug",
		"http": {"address": ":8080"},
		"database": {"host": "127.0.0.1", "port": 5432, "user": "app", "name": "stats", "options": {"sslmode": "disable"}}
	}`
	securityData := strings.TrimSpace(`
database:
  password: db-secret
youtube_api_key: yt-secret
`) + "\n"

	if err := os.WriteFile(configPath, []byte(configData), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(securityPath, []byte(securityData), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(
		base.WithConfigPath(configPath),
		base.WithSecurityPath(securityPath),
		base.WithEnvironment(false),
	)
	if err != nil {
		t.Fatal(err)
	}

	if cfg.LoggerLevel != "debug" {
		t.Fatalf("LoggerLevel = %q, want debug", cfg.LoggerLevel)
	}
	if cfg.HTTP.Address != ":8080" {
		t.Fatalf("HTTP.Address = %q, want :8080", cfg.HTTP.Address)
	}
	if cfg.Database.Host != "127.0.0.1" {
		t.Fatalf("Database.Host = %q, want 127.0.0.1", cfg.Database.Host)
	}
	if got := cfg.Database.Password.String(); got != "db-secret" {
		t.Fatalf("Database.Password = %q, want db-secret", got)
	}
	if got := cfg.YouTubeAPIKey.String(); got != "yt-secret" {
		t.Fatalf("YouTubeAPIKey = %q, want yt-secret", got)
	}
}

func TestLoaderSaveKeepsSecretsOutOfConfigJSON(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.json")
	securityPath := filepath.Join(dir, ".security.yml")

	loader, err := NewLoader(MainConfig,
		base.WithConfigPath(configPath),
		base.WithSecurityPath(securityPath),
		base.WithEnvironment(false),
	)
	if err != nil {
		t.Fatal(err)
	}

	cfg := DefaultMain()
	cfg.LoggerLevel = "info"
	cfg.HTTP.Address = ":9090"
	cfg.Database.Host = "db"
	cfg.Database.Port = 5432
	cfg.Database.User = "app"
	cfg.Database.Password = *NewSecureString("db-secret")
	cfg.Database.Name = "stats"
	cfg.YouTubeAPIKey = *NewSecureString("yt-secret")

	if err := loader.Save(cfg); err != nil {
		t.Fatal(err)
	}

	configBytes, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	configText := string(configBytes)
	if strings.Contains(configText, "db-secret") || strings.Contains(configText, "yt-secret") {
		t.Fatalf("config.json contains secrets: %s", configText)
	}

	var savedConfig map[string]any
	readJSON(t, configPath, &savedConfig)
	if savedConfig["logger_level"] != "info" {
		t.Fatalf("logger_level = %#v, want info", savedConfig["logger_level"])
	}

	securityBytes, err := os.ReadFile(securityPath)
	if err != nil {
		t.Fatal(err)
	}
	securityText := string(securityBytes)
	if !strings.Contains(securityText, "db-secret") || !strings.Contains(securityText, "yt-secret") {
		t.Fatalf("security file does not contain saved secrets: %s", securityText)
	}
	if strings.Contains(securityText, ":9090") || strings.Contains(securityText, "logger_level") {
		t.Fatalf("security file contains non-secret config: %s", securityText)
	}
}

func TestLoadHTTPConfig(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.json")
	securityPath := filepath.Join(dir, ".security.yml")

	configData := `{
		"logger_level": "info",
		"http": {"address": ":8081"},
		"web": {"root": "web/dist"}
	}`

	if err := os.WriteFile(configPath, []byte(configData), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(securityPath, []byte("\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	loader, err := NewLoader(HTTPConfig,
		base.WithConfigPath(configPath),
		base.WithSecurityPath(securityPath),
		base.WithEnvironment(false),
	)
	if err != nil {
		t.Fatal(err)
	}

	cfg, err := loader.Load()
	if err != nil {
		t.Fatal(err)
	}

	if cfg.LoggerLevel != "info" {
		t.Fatalf("LoggerLevel = %q, want info", cfg.LoggerLevel)
	}
	if cfg.HTTP.Address != ":8081" {
		t.Fatalf("HTTP.Address = %q, want :8081", cfg.HTTP.Address)
	}
	if cfg.Web.Root != "web/dist" {
		t.Fatalf("Web.Root = %q, want web/dist", cfg.Web.Root)
	}
}

func TestLoadTelegramConfig(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.json")
	securityPath := filepath.Join(dir, ".security.yml")

	configData := `{"logger_level": "debug"}`
	securityData := strings.TrimSpace(`
	token: tg-secret
	`) + "\n"

	if err := os.WriteFile(configPath, []byte(configData), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(securityPath, []byte(securityData), 0o600); err != nil {
		t.Fatal(err)
	}

	loader, err := NewLoader(TelegramConfig,
		base.WithConfigPath(configPath),
		base.WithSecurityPath(securityPath),
		base.WithEnvironment(false),
	)
	if err != nil {
		t.Fatal(err)
	}

	cfg, err := loader.Load()
	if err != nil {
		t.Fatal(err)
	}

	if cfg.LoggerLevel != "debug" {
		t.Fatalf("LoggerLevel = %q, want debug", cfg.LoggerLevel)
	}
	if got := cfg.Token.String(); got != "tg-secret" {
		t.Fatalf("Token = %q, want tg-secret", got)
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
