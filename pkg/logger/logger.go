package logger

import (
	"context"
	"fmt"
	multi "github.com/samber/slog-multi"
	"github.com/shopspring/decimal"
	"gopkg.in/natefinch/lumberjack.v2"
	"log/slog"
	"os"
	"runtime"
)

const (
	LevelTrace = slog.Level(-8)
	LevelFatal = slog.Level(12)
)

type Logger struct {
	log        *slog.Logger
	level      *slog.LevelVar
	levelNames map[slog.Leveler]string
}

func New() *Logger {
	l := &Logger{
		level: &slog.LevelVar{},
		levelNames: map[slog.Leveler]string{
			LevelTrace: "TRACE",
			LevelFatal: "FATAL",
		},
	}
	l.level.Set(slog.LevelInfo)

	opts := &slog.HandlerOptions{
		AddSource: true,
		Level:     l.level,
		ReplaceAttr: func(_ []string, a slog.Attr) slog.Attr {
			if a.Key == slog.LevelKey {
				level, ok := a.Value.Any().(slog.Level)
				if !ok {
					return a
				}
				levelLabel, exists := l.levelNames[level]
				if !exists {
					levelLabel = level.String()
				}

				a.Value = slog.StringValue(levelLabel)
			}
			if a.Key == "source" {
				_, file, line, ok := runtime.Caller(10)
				if ok {
					a.Value = slog.StringValue(fmt.Sprintf("%s:%d", file, line))
				}
			}

			return a
		},
	}

	logFile := &lumberjack.Logger{
		Filename:   "logs/main.log",
		MaxSize:    64,
		MaxBackups: 32,
		MaxAge:     30,
		Compress:   true,
	}

	l.log = slog.New(
		multi.Fanout(
			slog.NewTextHandler(os.Stdout, opts),
			slog.NewJSONHandler(logFile, opts),
		),
	)

	return l
}

func (l *Logger) SetLogLevel(levelStr string) {
	switch levelStr {
	case "trace":
		l.level.Set(LevelTrace)
	case "debug":
		l.level.Set(slog.LevelDebug)
	case "info":
		l.level.Set(slog.LevelInfo)
	case "warn":
		l.level.Set(slog.LevelWarn)
	case "error":
		l.level.Set(slog.LevelError)
	case "fatal":
		l.level.Set(LevelFatal)
	default:
		l.level.Set(slog.LevelInfo)
	}
}

func (l *Logger) GetLogLevel() string {
	switch l.level.Level() {
	case LevelTrace:
		return "trace"
	case slog.LevelDebug:
		return "debug"
	case slog.LevelInfo:
		return "info"
	case slog.LevelWarn:
		return "warn"
	case slog.LevelError:
		return "error"
	case LevelFatal:
		return "fatal"
	}

	return "info"
}

func (l *Logger) Trace(msg string, args ...any) {
	l.log.Log(context.Background(), LevelTrace, msg, args...)
}

func (l *Logger) Debug(msg string, args ...any) {
	l.log.Debug(msg, args...)
}

func (l *Logger) Info(msg string, args ...any) {
	l.log.Info(msg, args...)
}

func (l *Logger) Warn(msg string, args ...any) {
	l.log.Warn(msg, args...)
}

func (l *Logger) Error(msg string, err error, args ...any) {
	if err != nil {
		l.log.Error(msg, append([]any{slog.Any("error", err.Error())}, args...)...)
	} else {
		l.log.Error(msg, args...)
	}
}

func (l *Logger) Fatal(msg string, err error, args ...any) {
	l.log.Log(context.Background(), LevelFatal, msg, append([]any{slog.Any("error", err.Error())}, args...)...)
	os.Exit(1)
}

func Uint(key string, v uint) slog.Attr {
	return slog.Uint64(key, uint64(v))
}

func Decimal(key string, v decimal.Decimal) slog.Attr {
	return slog.String(key, v.String())
}
