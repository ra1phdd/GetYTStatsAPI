package core_telegram_middleware

import (
	"fmt"
	"runtime/debug"
	"time"

	core_telegram_tgctx "getytstatsapi/internal/core/transport/telegram/tgctx"

	"github.com/google/uuid"
	"github.com/ra1phdd/logger"
	tele "gopkg.in/telebot.v4"
)

const requestIDKey = "request_id"
const loggerKey = "logger"

func RequestID() Middleware {
	return func(next tele.HandlerFunc) tele.HandlerFunc {
		return func(c tele.Context) error {
			if c != nil {
				c.Set(requestIDKey, uuid.NewString())
			}

			return next(c)
		}
	}
}

func RequestIDFromContext(c tele.Context) (string, bool) {
	if c == nil {
		return "", false
	}

	requestID, ok := c.Get(requestIDKey).(string)
	return requestID, ok
}

func Logger(log *logger.Logger) Middleware {
	if log == nil {
		log = logger.New(logger.WithComponent("telegram.middleware"))
	}

	return func(next tele.HandlerFunc) tele.HandlerFunc {
		return func(c tele.Context) error {
			requestLog := log.With(core_telegram_tgctx.MessageAttrs(c)...)

			if c != nil {
				c.Set(loggerKey, requestLog)
			}

			return next(c)
		}
	}
}

func LoggerFromContext(c tele.Context) *logger.Logger {
	if c == nil {
		return nil
	}

	log, _ := c.Get(loggerKey).(*logger.Logger)
	return log
}

func Panic() Middleware {
	return func(next tele.HandlerFunc) tele.HandlerFunc {
		return func(c tele.Context) (err error) {
			log := LoggerFromContext(c)
			if log == nil {
				log = logger.New(logger.WithComponent("telegram.middleware"))
			}

			defer func() {
				if p := recover(); p != nil {
					log.Error(
						"during handle Telegram update got unexpected panic",
						logger.String("panic", fmt.Sprint(p)),
						logger.String("stack", string(debug.Stack())),
					)
					err = nil
				}
			}()

			return next(c)
		}
	}
}

func Trace() Middleware {
	return func(next tele.HandlerFunc) tele.HandlerFunc {
		return func(c tele.Context) error {
			log := LoggerFromContext(c)
			if log == nil {
				log = logger.New(logger.WithComponent("telegram.middleware"))
			}

			before := time.Now()
			log.Debug(
				">>> incoming Telegram update",
				logger.Time("time", before.UTC()),
			)

			err := next(c)

			attrs := []logger.Attr{
				logger.Duration("latency", time.Since(before)),
			}
			if err != nil {
				attrs = append(attrs, logger.Err(err))
			}

			log.Debug("<<< done Telegram update", attrs...)
			return err
		}
	}
}
