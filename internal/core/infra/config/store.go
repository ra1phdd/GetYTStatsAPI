package config

import (
	"sync"

	base "getytstatsapi/pkg/config"
)

type Store struct {
	cfg    *Config
	loader *base.Loader
	mu     sync.RWMutex
}

func NewStore(cfg *Config, loader *base.Loader) *Store {
	if cfg != nil {
		cfg.EnsureData()
	}
	return &Store{
		cfg:    cfg,
		loader: loader,
	}
}

func (s *Store) Read(fn func(*Config) error) error {
	if s == nil || s.cfg == nil {
		return ErrNilStore
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return fn(s.cfg)
}

func (s *Store) Update(fn func(*Config) error) error {
	if s == nil || s.cfg == nil {
		return ErrNilStore
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.cfg.EnsureData()
	if err := fn(s.cfg); err != nil {
		return err
	}
	if s.loader == nil {
		return ErrNilStore
	}
	snapshot := *s.cfg
	snapshot.EnsureData()
	if err := SaveRuntimeRefs(&snapshot, s.loader.ConfigPath()); err != nil {
		return err
	}
	return s.loader.Save(&snapshot)
}
