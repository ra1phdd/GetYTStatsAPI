package main

import (
	"context"
	core_config "getytstatsapi/internal/core/infra/config"
	core_http_middleware "getytstatsapi/internal/core/transport/http/middleware"
	core_http_server "getytstatsapi/internal/core/transport/http/server"
	core_telegram_input "getytstatsapi/internal/core/transport/telegram/input"
	core_telegram_middleware "getytstatsapi/internal/core/transport/telegram/middleware"
	core_telegram_server "getytstatsapi/internal/core/transport/telegram/server"
	campaignapi_client "getytstatsapi/internal/entities/campaign/client/api"
	campaign_telegram "getytstatsapi/internal/entities/campaign/transport/telegram"
	"net/http"
	"os/signal"
	"syscall"

	"github.com/ra1phdd/logger"
	tele "gopkg.in/telebot.v4"
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

	apiClient := campaignapi_client.New(cfg.API.BaseURL, cfg.Internal.ServiceID, cfg.Internal.ServiceSecret.String())
	stateStore := campaignapi_client.NewInputStateStore(context.Background(), apiClient)
	inputRouter := core_telegram_input.NewInputWithStore(log.Named("input"), stateStore)
	handler := campaign_telegram.NewHandler(log.Named("handler"), apiClient, inputRouter)
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
		core_telegram_server.NewRoute(tele.OnText, handler.OnText),
		core_telegram_server.NewRoute(tele.OnCallback, handler.OnCallback),
	)

	webhookHandler := campaign_telegram.NewWebhookHandler(
		log.Named("webhook"),
		server.Bot(),
		cfg.Internal.PeerServiceID,
		cfg.Internal.PeerServiceSecret.String(),
	)
	webhookRouter := core_http_server.NewAPIVersionRouter(core_http_server.ApiVersion1)
	webhookRouter.RegisterRoutes(
		core_http_server.NewRoute(http.MethodPost, "/internal/webhooks/campaign-events", webhookHandler.ServeHTTP),
	)
	webhookServer := core_http_server.NewHTTPServer(
		cfg.Webhook.Address,
		log.Named("webhook.http"),
		core_http_middleware.RequestID(),
		core_http_middleware.Logger(log.Named("webhook.middleware")),
		core_http_middleware.Trace(),
		core_http_middleware.Panic(),
	)
	webhookServer.RegisterAPIRouters(webhookRouter)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		if err := webhookServer.Run(ctx); err != nil {
			panic(err)
		}
	}()

	if err := server.Run(ctx); err != nil {
		panic(err)
	}
}
