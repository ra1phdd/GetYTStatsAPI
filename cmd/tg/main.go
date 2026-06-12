package main

import (
	"context"
	core_config "getytstatsapi/internal/core/infra/config"
	core_telegram_middleware "getytstatsapi/internal/core/transport/telegram/middleware"
	core_telegram_server "getytstatsapi/internal/core/transport/telegram/server"
	start_telegram "getytstatsapi/internal/entities/start/transport/telegram"
	"os/signal"
	"syscall"

	"github.com/ra1phdd/logger"
)

func main() {
	loader, err := core_config.NewLoader(core_config.TelegramConfig)
	if err != nil {
		panic(err)
	}

	cfg, err := loader.Load()
	if err != nil {
		panic(err)
	}

	log := logger.New(
		logger.WithComponent("telegram"),
		logger.WithLevelString(cfg.LoggerLevel),
	)

	handler := start_telegram.NewHandler(log.Named("handler"))
	server, err := core_telegram_server.NewServer(
		cfg.Token.String(),
		log.Named("server"),
		core_telegram_middleware.RequestID(),
		core_telegram_middleware.Logger(log.Named("middleware")),
		core_telegram_middleware.Trace(),
		core_telegram_middleware.Panic(),
	)
	if err != nil {
		panic(err)
	}

	server.RegisterRoutes(
		core_telegram_server.NewRoute("/start", handler.Start),
	)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := server.Run(ctx); err != nil {
		panic(err)
	}
}
