package core_telegram_server

import (
	"context"
	"fmt"
	core_telegram_middleware "getytstatsapi/internal/core/transport/telegram/middleware"
	"strings"
	"time"

	"github.com/ra1phdd/logger"
	tele "gopkg.in/telebot.v4"
)

type Server struct {
	bot        *tele.Bot
	log        *logger.Logger
	middleware []core_telegram_middleware.Middleware
}

func NewServer(
	token string,
	log *logger.Logger,
	middleware ...core_telegram_middleware.Middleware,
) (*Server, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return nil, fmt.Errorf("telegram token is required")
	}
	if log == nil {
		log = logger.New(logger.WithComponent("telegram.server"))
	}

	bot, err := tele.NewBot(tele.Settings{
		Token:  token,
		Poller: &tele.LongPoller{Timeout: time.Second},
	})
	if err != nil {
		return nil, fmt.Errorf("create telegram bot: %w", err)
	}

	return &Server{
		bot:        bot,
		log:        log,
		middleware: middleware,
	}, nil
}

func (s *Server) RegisterRoutes(routes ...Route) {
	if s == nil || s.bot == nil {
		return
	}

	for _, route := range routes {
		if strings.TrimSpace(route.Command) == "" || route.Handler == nil {
			continue
		}

		handler := core_telegram_middleware.ChainMiddleware(route.Handler, s.middleware...)
		s.bot.Handle(route.Command, handler)
	}
}

func (s *Server) Run(ctx context.Context) error {
	if s == nil || s.bot == nil {
		return fmt.Errorf("telegram server is not initialized")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	stopped := make(chan struct{})
	go func() {
		defer close(stopped)
		s.log.Info("start telegram bot")
		s.bot.Start()
	}()

	select {
	case <-ctx.Done():
		s.log.Warn("shutdown telegram bot", logger.Err(ctx.Err()))
		s.bot.Stop()
		<-stopped
		s.log.Warn("telegram bot stopped")
		return nil

	case <-stopped:
		s.log.Warn("telegram bot stopped")
		return nil
	}
}
