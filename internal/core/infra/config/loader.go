package config

import (
	"fmt"

	base "github.com/ra1phdd/config"
)

type Schema[T any] struct {
	defaults func() *T
}

var MainConfig = Schema[Main]{defaults: DefaultMain}
var HTTPConfig = Schema[HTTPServer]{defaults: DefaultHTTP}
var TelegramConfig = Schema[Telegram]{defaults: DefaultTelegram}

type Loader[T any] struct {
	base   *base.Loader
	schema Schema[T]
}

func NewLoader[T any](schema Schema[T], options ...base.Option) (*Loader[T], error) {
	if schema.defaults == nil {
		return nil, fmt.Errorf("%w: config schema is required", base.ErrInvalidConfig)
	}

	baseOptions := []base.Option{
		base.WithConfigPath(ConfigPath),
		base.WithSecurityPath(SecurityPath),
	}

	loader, err := base.NewLoader(append(baseOptions, options...)...)
	if err != nil {
		return nil, err
	}

	return &Loader[T]{
		base:   loader,
		schema: schema,
	}, nil
}

func Load(options ...base.Option) (*Config, error) {
	loader, err := NewLoader(MainConfig, options...)
	if err != nil {
		return nil, err
	}

	return loader.Load()
}

func (l *Loader[T]) Load() (*T, error) {
	if l == nil || l.base == nil || l.schema.defaults == nil {
		return nil, base.ErrNilConfig
	}

	cfg := l.schema.defaults()
	if err := l.base.LoadInto(cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

func (l *Loader[T]) Save(cfg *T) error {
	if l == nil || l.base == nil {
		return base.ErrNilConfig
	}

	return l.base.Save(cfg)
}

func (l *Loader[T]) ConfigPath() string {
	if l == nil || l.base == nil {
		return ""
	}

	return l.base.ConfigPath()
}

func (l *Loader[T]) SecurityPath() string {
	if l == nil || l.base == nil {
		return ""
	}

	return l.base.SecurityPath()
}
