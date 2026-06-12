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
		"database": {"host": "127.0.0.1", "port": 5432, "user": "app", "name": "stats", "options": {"sslmode": "disable"}},
		"notifications": {"webhook_url": "http://127.0.0.1:8081"},
		"internal": {"api": {"service_id": "campaign-api", "peer_service_id": "telegram-bot"}},
		"public_base_url": "http://127.0.0.1:8080"
	}`
	securityData := strings.TrimSpace(`
database:
  password: db-secret
youtube_api_key: yt-secret
telegram:
  bot_token: tg-secret
internal:
  api:
    service_secret: api-secret
    peer_service_secret: bot-secret
  bot:
    service_secret: bot-secret
    peer_service_secret: api-secret
export_jwt_secret: export-secret
access_jwt_secret: access-secret
refresh_jwt_secret: refresh-secret
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
	cfg.Telegram.BotToken = *NewSecureString("tg-secret")
	cfg.Notifications.WebhookURL = "http://127.0.0.1:8081"
	cfg.Internal.API.ServiceID = "campaign-api"
	cfg.Internal.API.ServiceSecret = *NewSecureString("api-secret")
	cfg.Internal.API.PeerServiceID = "telegram-bot"
	cfg.Internal.API.PeerServiceSecret = *NewSecureString("bot-secret")
	cfg.Internal.Bot.ServiceID = "telegram-bot"
	cfg.Internal.Bot.ServiceSecret = *NewSecureString("bot-secret")
	cfg.Internal.Bot.PeerServiceID = "campaign-api"
	cfg.Internal.Bot.PeerServiceSecret = *NewSecureString("api-secret")
	cfg.PublicBaseURL = "http://127.0.0.1:8080"
	cfg.ExportJWTSecret = *NewSecureString("export-secret")
	cfg.AccessJWTSecret = *NewSecureString("access-secret")
	cfg.RefreshJWTSecret = *NewSecureString("refresh-secret")

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

	configData := `{
		"logger_level": "debug",
		"http": {"address": ":8081"},
		"notifications": {"webhook_url": "http://127.0.0.1:8081"},
		"api": {"base_url": "http://127.0.0.1:8080"},
		"internal": {"bot": {"service_id": "telegram-bot", "peer_service_id": "campaign-api"}}
	}`
	securityData := "telegram:\n  bot_token: tg-secret\ninternal:\n  bot:\n    service_secret: bot-secret\n    peer_service_secret: api-secret\n"

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
	if cfg.HTTP.Address != ":8081" {
		t.Fatalf("HTTP.Address = %q, want :8081", cfg.HTTP.Address)
	}
	if got := cfg.Telegram.BotToken.String(); got != "tg-secret" {
		t.Fatalf("Telegram.BotToken = %q, want tg-secret", got)
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
