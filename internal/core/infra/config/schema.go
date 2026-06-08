package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	base "getytstatsapi/pkg/config"
)

type SecureString = base.SecureString

var NewSecureString = base.NewSecureString

type Config struct {
	LoggerLevel string          `json:"logger_level" yaml:"logger_level" env:"LOGGER_LEVEL"`
	HTTP        HTTPConfig      `json:"http" yaml:"http" envPrefix:"HTTP_"`
	Database    Database        `json:"database" yaml:"database" envPrefix:"DATABASE_"`
	Features    FeaturesConfig  `json:"features" yaml:"features" envPrefix:"FEATURES_"`
	ChatUsers   ChatUsersConfig `json:"chat_users,omitempty" yaml:"chat_users,omitempty"`
	runtimeRefs runtimeRefs
}

type runtimeRefs struct {
	ChatUsers string
}

type HTTPConfig struct {
	Address string `json:"address" yaml:"address" env:"ADDRESS"`
	BaseURL string `json:"base_url,omitempty" yaml:"base_url,omitempty" env:"BASE_URL"`
}

type Database struct {
	Host     string            `json:"host" yaml:"host" env:"HOST"`
	Port     int               `json:"port" yaml:"port" env:"PORT"`
	User     string            `json:"user" yaml:"user" env:"USER"`
	Password SecureString      `json:"password,omitzero" yaml:"password,omitempty" env:"PASSWORD"`
	Name     string            `json:"name" yaml:"name" env:"NAME"`
	Options  map[string]string `json:"options,omitempty" yaml:"options,omitempty"`
}

type FeaturesConfig struct {
	StintInside StintInsideFeatureConfig `json:"stintinside" yaml:"stintinside" envPrefix:"STINTINSIDE_"`
}

type StintInsideFeatureConfig struct {
	YouTubeAPIKey SecureString `json:"youtube_api_key,omitzero" yaml:"youtube_api_key,omitempty" env:"YOUTUBE_API_KEY"`
}

type ChatUsersConfig map[int64]map[int64]KnownUser

type KnownUser struct {
	ID        int64  `json:"id" yaml:"id"`
	FirstName string `json:"first_name,omitempty" yaml:"first_name,omitempty"`
	LastName  string `json:"last_name,omitempty" yaml:"last_name,omitempty"`
	Username  string `json:"username,omitempty" yaml:"username,omitempty"`
	IsBot     bool   `json:"is_bot,omitempty" yaml:"is_bot,omitempty"`
}

func (d Database) DSN() string {
	q := url.Values{}
	for k, v := range d.Options {
		key := strings.TrimSpace(strings.ToLower(k))
		value := strings.TrimSpace(v)
		if key == "" || value == "" {
			continue
		}

		if key == "prefer_simple_protocol" {
			continue
		}

		q.Set(key, value)
	}

	u := &url.URL{
		Scheme:   "postgres",
		Host:     d.Host + ":" + strconv.Itoa(d.Port),
		Path:     "/" + d.Name,
		RawQuery: q.Encode(),
	}
	if d.User != "" {
		u.User = url.UserPassword(d.User, d.Password.String())
	}
	return u.String()
}

func (c *Config) Validate() error {
	if c == nil {
		return base.ErrNilConfig
	}
	if strings.TrimSpace(c.HTTP.Address) == "" {
		return fmt.Errorf("%w: http.address is required", base.ErrInvalidConfig)
	}
	if c.Database.Port < 0 {
		return fmt.Errorf("%w: database.port cannot be negative", base.ErrInvalidConfig)
	}
	if strings.TrimSpace(c.Database.Host) == "" {
		return fmt.Errorf("%w: database.host is required", base.ErrInvalidConfig)
	}
	if strings.TrimSpace(c.Database.User) == "" {
		return fmt.Errorf("%w: database.user is required", base.ErrInvalidConfig)
	}
	if strings.TrimSpace(c.Database.Password.String()) == "" {
		return fmt.Errorf("%w: database.password is required", base.ErrInvalidConfig)
	}
	if strings.TrimSpace(c.Database.Name) == "" {
		return fmt.Errorf("%w: database.name is required", base.ErrInvalidConfig)
	}
	return nil
}

func (c *Config) EnsureData() {
	if c.ChatUsers == nil {
		c.ChatUsers = make(ChatUsersConfig)
	}
}

func (c *Config) MarshalJSON() ([]byte, error) {
	if c != nil {
		c.EnsureData()
	}
	type Alias Config
	type runtimeAlias struct {
		*Alias

		ChatUsers any `json:"chat_users,omitempty"`
	}
	chatUsers := any(c.ChatUsers)
	if c.runtimeRefs.ChatUsers != "" {
		chatUsers = c.runtimeRefs.ChatUsers
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(runtimeAlias{Alias: (*Alias)(c), ChatUsers: chatUsers}); err != nil {
		return nil, err
	}
	return bytes.TrimRight(buf.Bytes(), "\n"), nil
}

func (c *Config) UnmarshalJSON(data []byte) error {
	type Alias Config
	aux := struct {
		*Alias

		ChatUsers json.RawMessage `json:"chat_users"`
	}{Alias: (*Alias)(c)}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	if len(aux.ChatUsers) > 0 && string(aux.ChatUsers) != "null" {
		ref, ok, err := decodeRuntimeRef(aux.ChatUsers)
		if err != nil {
			return fmt.Errorf("chat_users: %w", err)
		}
		if ok {
			c.runtimeRefs.ChatUsers = ref
			c.ChatUsers = nil
		} else if err := json.Unmarshal(aux.ChatUsers, &c.ChatUsers); err != nil {
			return fmt.Errorf("chat_users: %w", err)
		}
	}
	return nil
}

func decodeRuntimeRef(data []byte) (string, bool, error) {
	var ref string
	if err := json.Unmarshal(data, &ref); err != nil {
		return "", false, err
	}

	ref = strings.TrimSpace(ref)
	if ref == "" {
		return "", false, nil
	}

	if !strings.HasPrefix(ref, "file://") {
		return "", false, errors.New("only file:// references are supported")
	}
	return ref, true, nil
}

func DefaultConfig() *Config {
	cfg := &Config{
		LoggerLevel: "warn",
		HTTP: HTTPConfig{
			Address: ":80",
		},
		Database: Database{
			Host: "127.0.0.1",
			Port: 5432,
			Options: map[string]string{
				"sslmode":                "disable",
				"prefer_simple_protocol": "true",
			},
		},
	}
	cfg.EnsureData()
	return cfg
}
