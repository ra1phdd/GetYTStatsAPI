package logger

import (
	"io"
	"log/slog"
	"os"
	"strings"
)

type Option func(*config)

type config struct {
	level       slog.Level
	stdout      io.Writer
	jsonOutput  io.Writer
	filePath    string
	component   string
	timeFormat  string
	noColor     bool
	disableFile bool
	exit        func(int)
}

func defaultConfig() config {
	return config{
		level:      slog.LevelInfo,
		stdout:     os.Stdout,
		filePath:   defaultLogFile,
		timeFormat: defaultTimeFormat,
		exit:       os.Exit,
	}
}

func WithLevel(level slog.Level) Option {
	return func(cfg *config) {
		cfg.level = level
	}
}

func WithLevelString(level string) Option {
	return func(cfg *config) {
		if parsed, ok := parseLevel(level); ok {
			cfg.level = parsed
		}
	}
}

func WithOutput(w io.Writer) Option {
	return func(cfg *config) {
		if w != nil {
			cfg.stdout = w
		}
	}
}

func WithJSONOutput(w io.Writer) Option {
	return func(cfg *config) {
		cfg.jsonOutput = w
		cfg.disableFile = w == nil
	}
}

func WithFile(path string) Option {
	return func(cfg *config) {
		if path != "" {
			cfg.filePath = path
			cfg.disableFile = false
		}
	}
}

func WithoutFile() Option {
	return func(cfg *config) {
		cfg.jsonOutput = nil
		cfg.disableFile = true
	}
}

func WithComponent(component string) Option {
	return func(cfg *config) {
		cfg.component = strings.TrimSpace(component)
	}
}

func WithTimeFormat(format string) Option {
	return func(cfg *config) {
		if format != "" {
			cfg.timeFormat = format
		}
	}
}

func WithNoColor(noColor bool) Option {
	return func(cfg *config) {
		cfg.noColor = noColor
	}
}

func WithExit(exit func(int)) Option {
	return func(cfg *config) {
		if exit != nil {
			cfg.exit = exit
		}
	}
}
