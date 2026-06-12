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
	LoggerLevel      string        `json:"logger_level" yaml:"logger_level" env:"LOGGER_LEVEL"`
	HTTP             HTTP          `json:"http" yaml:"http" envPrefix:"API_HTTP_"`
	Database         Database      `json:"database" yaml:"database" envPrefix:"DATABASE_"`
	YouTubeAPIKey    SecureString  `json:"youtube_api_key,omitzero" yaml:"youtube_api_key,omitempty" env:"YOUTUBE_API_KEY"`
	GoogleOAuth      GoogleOAuth   `json:"google_oauth" yaml:"google_oauth" envPrefix:"GOOGLE_OAUTH_"`
	Telegram         TelegramBot   `json:"telegram" yaml:"telegram" envPrefix:"TELEGRAM_"`
	Notifications    Notifications `json:"notifications" yaml:"notifications" envPrefix:"NOTIFICATIONS_"`
	Internal         InternalApps  `json:"internal" yaml:"internal" envPrefix:"INTERNAL_"`
	PublicBaseURL    string        `json:"public_base_url" yaml:"public_base_url" env:"PUBLIC_BASE_URL"`
	ExportJWTSecret  SecureString  `json:"export_jwt_secret,omitzero" yaml:"export_jwt_secret,omitempty" env:"EXPORT_JWT_SECRET"`
	AccessJWTSecret  SecureString  `json:"access_jwt_secret,omitzero" yaml:"access_jwt_secret,omitempty" env:"ACCESS_JWT_SECRET"`
	RefreshJWTSecret SecureString  `json:"refresh_jwt_secret,omitzero" yaml:"refresh_jwt_secret,omitempty" env:"REFRESH_JWT_SECRET"`
}

type HTTPServer struct {
	LoggerLevel string `json:"logger_level" yaml:"logger_level" env:"LOGGER_LEVEL"`
	HTTP        HTTP   `json:"http" yaml:"http" envPrefix:"HTTP_"`
	Web         Web    `json:"web" yaml:"web" envPrefix:"WEB_"`
}

type Telegram struct {
	LoggerLevel   string        `json:"logger_level" yaml:"logger_level" env:"LOGGER_LEVEL"`
	Telegram      TelegramBot   `json:"telegram" yaml:"telegram" envPrefix:"TELEGRAM_"`
	HTTP          HTTP          `json:"http" yaml:"http" envPrefix:"TELEGRAM_HTTP_"`
	API           APIClient     `json:"api" yaml:"api" envPrefix:"API_"`
	Notifications Notifications `json:"notifications" yaml:"notifications" envPrefix:"NOTIFICATIONS_"`
	Internal      InternalApps  `json:"internal" yaml:"internal" envPrefix:"INTERNAL_"`
}

type TelegramBot struct {
	BotToken SecureString `json:"bot_token,omitzero" yaml:"bot_token,omitempty" env:"BOT_TOKEN"`
}

type GoogleOAuth struct {
	ClientID     string       `json:"client_id" yaml:"client_id" env:"CLIENT_ID"`
	ClientSecret SecureString `json:"client_secret,omitzero" yaml:"client_secret,omitempty" env:"CLIENT_SECRET"`
	RedirectURL  string       `json:"redirect_url" yaml:"redirect_url" env:"REDIRECT_URL"`
}

type Notifications struct {
	WebhookURL string `json:"webhook_url" yaml:"webhook_url" env:"WEBHOOK_URL"`
}

type APIClient struct {
	BaseURL string `json:"base_url" yaml:"base_url" env:"BASE_URL"`
}

type ServiceAuth struct {
	ServiceID         string       `json:"service_id" yaml:"service_id" env:"SERVICE_ID"`
	ServiceSecret     SecureString `json:"service_secret,omitzero" yaml:"service_secret,omitempty" env:"SERVICE_SECRET"`
	PeerServiceID     string       `json:"peer_service_id" yaml:"peer_service_id" env:"PEER_SERVICE_ID"`
	PeerServiceSecret SecureString `json:"peer_service_secret,omitzero" yaml:"peer_service_secret,omitempty" env:"PEER_SERVICE_SECRET"`
}

type InternalApps struct {
	API ServiceAuth `json:"api" yaml:"api" envPrefix:"API_"`
	Bot ServiceAuth `json:"bot" yaml:"bot" envPrefix:"BOT_"`
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
	if strings.TrimSpace(c.Telegram.BotToken.String()) == "" {
		return fmt.Errorf("%w: telegram.bot_token is required", base.ErrInvalidConfig)
	}
	if strings.TrimSpace(c.HTTP.Address) == "" {
		return fmt.Errorf("%w: api http.address is required", base.ErrInvalidConfig)
	}
	if strings.TrimSpace(c.Notifications.WebhookURL) == "" {
		return fmt.Errorf("%w: notifications.webhook_url is required", base.ErrInvalidConfig)
	}
	if strings.TrimSpace(c.GoogleOAuth.ClientID) != "" || strings.TrimSpace(c.GoogleOAuth.ClientSecret.String()) != "" || strings.TrimSpace(c.GoogleOAuth.RedirectURL) != "" {
		if strings.TrimSpace(c.GoogleOAuth.ClientID) == "" || strings.TrimSpace(c.GoogleOAuth.ClientSecret.String()) == "" {
			return fmt.Errorf("%w: google_oauth.client_id and google_oauth.client_secret must be configured together", base.ErrInvalidConfig)
		}
	}
	if strings.TrimSpace(c.Internal.API.ServiceID) == "" || strings.TrimSpace(c.Internal.API.ServiceSecret.String()) == "" {
		return fmt.Errorf("%w: internal.api service credentials are required", base.ErrInvalidConfig)
	}
	if strings.TrimSpace(c.Internal.API.PeerServiceID) == "" || strings.TrimSpace(c.Internal.API.PeerServiceSecret.String()) == "" {
		return fmt.Errorf("%w: internal.api peer service credentials are required", base.ErrInvalidConfig)
	}
	if strings.TrimSpace(c.PublicBaseURL) == "" {
		return fmt.Errorf("%w: public_base_url is required", base.ErrInvalidConfig)
	}
	if strings.TrimSpace(c.ExportJWTSecret.String()) == "" {
		return fmt.Errorf("%w: export_jwt_secret is required", base.ErrInvalidConfig)
	}
	if strings.TrimSpace(c.AccessJWTSecret.String()) == "" {
		return fmt.Errorf("%w: access_jwt_secret is required", base.ErrInvalidConfig)
	}
	if strings.TrimSpace(c.RefreshJWTSecret.String()) == "" {
		return fmt.Errorf("%w: refresh_jwt_secret is required", base.ErrInvalidConfig)
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
	if strings.TrimSpace(c.Telegram.BotToken.String()) == "" {
		return fmt.Errorf("%w: telegram.bot_token is required", base.ErrInvalidConfig)
	}
	if strings.TrimSpace(c.HTTP.Address) == "" {
		return fmt.Errorf("%w: telegram http.address is required", base.ErrInvalidConfig)
	}
	if strings.TrimSpace(c.API.BaseURL) == "" {
		return fmt.Errorf("%w: api.base_url is required", base.ErrInvalidConfig)
	}
	if strings.TrimSpace(c.Internal.Bot.ServiceID) == "" || strings.TrimSpace(c.Internal.Bot.ServiceSecret.String()) == "" {
		return fmt.Errorf("%w: internal.bot service credentials are required", base.ErrInvalidConfig)
	}
	if strings.TrimSpace(c.Internal.Bot.PeerServiceID) == "" || strings.TrimSpace(c.Internal.Bot.PeerServiceSecret.String()) == "" {
		return fmt.Errorf("%w: internal.bot peer service credentials are required", base.ErrInvalidConfig)
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
		PublicBaseURL: "http://127.0.0.1:8080",
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
		Telegram:    TelegramBot{},
		HTTP: HTTP{
			Address: ":8081",
		},
		API: APIClient{
			BaseURL: "http://127.0.0.1:8080",
		},
		Notifications: Notifications{
			WebhookURL: "http://127.0.0.1:8081",
		},
	}
}

type Config = Main

func DefaultConfig() *Config {
	return DefaultMain()
}
