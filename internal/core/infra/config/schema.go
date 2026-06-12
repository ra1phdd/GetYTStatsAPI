package config

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"

	base "github.com/ra1phdd/config"
)

type SecureString = base.SecureString

var NewSecureString = base.NewSecureString

type Main struct {
	LoggerLevel   string       `json:"logger_level" yaml:"logger_level" env:"LOGGER_LEVEL"`
	HTTP          HTTP         `json:"http" yaml:"http" envPrefix:"HTTP_"`
	Database      Database     `json:"database" yaml:"database" envPrefix:"DATABASE_"`
	YouTubeAPIKey SecureString `json:"youtube_api_key,omitzero" yaml:"youtube_api_key,omitempty" env:"YOUTUBE_API_KEY"`
}

type HTTPServer struct {
	LoggerLevel string `json:"logger_level" yaml:"logger_level" env:"LOGGER_LEVEL"`
	HTTP        HTTP   `json:"http" yaml:"http" envPrefix:"HTTP_"`
	Web         Web    `json:"web" yaml:"web" envPrefix:"WEB_"`
}

type Telegram struct {
	LoggerLevel string       `json:"logger_level" yaml:"logger_level" env:"LOGGER_LEVEL"`
	Token       SecureString `json:"token,omitzero" yaml:"token,omitempty" env:"TELEGRAM_TOKEN"`
}

type HTTP struct {
	Address string `json:"address" yaml:"address" env:"ADDRESS"`
}

type Web struct {
	Root string `json:"root" yaml:"root" env:"ROOT"`
}

type Database struct {
	Host     string            `json:"host" yaml:"host" env:"HOST"`
	Port     int               `json:"port" yaml:"port" env:"PORT"`
	User     string            `json:"user" yaml:"user" env:"USER"`
	Password SecureString      `json:"password,omitzero" yaml:"password,omitempty" env:"PASSWORD"`
	Name     string            `json:"name" yaml:"name" env:"NAME"`
	Options  map[string]string `json:"options,omitempty" yaml:"options,omitempty"`
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

func (c *Main) Validate() error {
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

func (c *HTTPServer) Validate() error {
	if c == nil {
		return base.ErrNilConfig
	}
	if strings.TrimSpace(c.HTTP.Address) == "" {
		return fmt.Errorf("%w: http.address is required", base.ErrInvalidConfig)
	}
	if strings.TrimSpace(c.Web.Root) == "" {
		return fmt.Errorf("%w: web.root is required", base.ErrInvalidConfig)
	}
	return nil
}

func (c *Telegram) Validate() error {
	if c == nil {
		return base.ErrNilConfig
	}
	if strings.TrimSpace(c.Token.String()) == "" {
		return fmt.Errorf("%w: token is required", base.ErrInvalidConfig)
	}
	return nil
}

func DefaultMain() *Main {
	cfg := &Main{
		LoggerLevel: "warn",
		HTTP: HTTP{
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

	return cfg
}

func DefaultHTTP() *HTTPServer {
	return &HTTPServer{
		LoggerLevel: "warn",
		HTTP: HTTP{
			Address: ":8080",
		},
		Web: Web{
			Root: "{PWD}/web/dist",
		},
	}
}

func DefaultTelegram() *Telegram {
	return &Telegram{
		LoggerLevel: "warn",
	}
}

type Config = Main

func DefaultConfig() *Config {
	return DefaultMain()
}
