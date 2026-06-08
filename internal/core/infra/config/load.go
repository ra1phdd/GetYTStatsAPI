package config

import base "getytstatsapi/pkg/config"

func LoadStore(options ...base.Option) (*Store, error) {
	loader, err := base.NewLoader(options...)
	if err != nil {
		return nil, err
	}

	cfg := DefaultConfig()
	if err := loader.LoadInto(cfg); err != nil {
		return nil, err
	}
	if err := LoadRuntimeRefs(cfg, loader.ConfigPath()); err != nil {
		return nil, err
	}

	cfg.EnsureData()
	return NewStore(cfg, loader), nil
}
