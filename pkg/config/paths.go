package config

import (
	"os"
	"path/filepath"
	"strings"
)

type Replacer struct {
	Getenv      func(string) string
	Getwd       func() (string, error)
	UserHomeDir func() (string, error)

	DefaultConfigPath   string
	DefaultAuthPath     string
	DefaultSecurityPath string
	EnvConfig           string
	EnvAuth             string
	EnvSecurity         string
}

func DefaultReplacer() Replacer {
	return Replacer{
		Getenv:      os.Getenv,
		Getwd:       os.Getwd,
		UserHomeDir: os.UserHomeDir,

		DefaultConfigPath:   ConfigPath,
		DefaultAuthPath:     AuthPath,
		DefaultSecurityPath: SecurityPath,
		EnvConfig:           EnvConfig,
		EnvAuth:             EnvAuth,
		EnvSecurity:         EnvSecurity,
	}
}

func (r Replacer) WithDefaults() Replacer {
	if r.Getenv == nil {
		r.Getenv = os.Getenv
	}
	if r.Getwd == nil {
		r.Getwd = os.Getwd
	}
	if r.UserHomeDir == nil {
		r.UserHomeDir = os.UserHomeDir
	}
	if r.DefaultConfigPath == "" {
		r.DefaultConfigPath = ConfigPath
	}
	if r.DefaultAuthPath == "" {
		r.DefaultAuthPath = AuthPath
	}
	if r.DefaultSecurityPath == "" {
		r.DefaultSecurityPath = SecurityPath
	}
	if r.EnvConfig == "" {
		r.EnvConfig = EnvConfig
	}
	if r.EnvAuth == "" {
		r.EnvAuth = EnvAuth
	}
	if r.EnvSecurity == "" {
		r.EnvSecurity = EnvSecurity
	}
	return r
}

func (r Replacer) ConfigPath() string {
	path := r.DefaultConfigPath
	if envPath := strings.TrimSpace(r.Getenv(r.EnvConfig)); envPath != "" {
		path = envPath
	}
	return r.CleanAbs(path)
}

func (r Replacer) AuthPath() string {
	path := r.DefaultAuthPath
	if envPath := strings.TrimSpace(r.Getenv(r.EnvAuth)); envPath != "" {
		path = envPath
	}
	return r.CleanAbs(path)
}

func (r Replacer) SecurityPath() string {
	path := r.DefaultSecurityPath
	if envPath := strings.TrimSpace(r.Getenv(r.EnvSecurity)); envPath != "" {
		path = envPath
	}
	return r.CleanAbs(path)
}

func (r Replacer) CleanAbs(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}

	expanded := r.ExpandMarkers(path)
	if !filepath.IsAbs(expanded) {
		var err error
		expanded, err = filepath.Abs(expanded)
		if err != nil {
			return path
		}
	}

	return filepath.Clean(expanded)
}

func (r Replacer) ExpandMarkers(path string) string {
	home, err := r.UserHomeDir()
	if err != nil {
		home = "."
	}

	cwd, err := r.Getwd()
	if err != nil {
		cwd = "."
	}

	replacer := strings.NewReplacer(
		"{HOME}", home,
		"{PWD}", cwd,
		"{CWD}", cwd,
	)
	return replacer.Replace(path)
}
