package main

import (
	"context"
	core_config "getytstatsapi/internal/core/infra/config"
	"getytstatsapi/internal/core/infra/postgres"
	core_http_middleware "getytstatsapi/internal/core/transport/http/middleware"
	core_http_server "getytstatsapi/internal/core/transport/http/server"
	health_repository "getytstatsapi/internal/entities/health/repository"
	health_service "getytstatsapi/internal/entities/health/service"
	health_http "getytstatsapi/internal/entities/health/transport/http"
	sponsorblock_repository "getytstatsapi/internal/entities/sponsorblock/repository/sponsorblock"
	postgres_stats_repository "getytstatsapi/internal/entities/stats/repository/postgres"
	youtube_stats_repository "getytstatsapi/internal/entities/stats/repository/youtube"
	stats_service "getytstatsapi/internal/entities/stats/service"
	stats_http "getytstatsapi/internal/entities/stats/transport/http"
	"net/http"
	"os/signal"
	"strings"
	"syscall"

	"github.com/ra1phdd/logger"
)

func main() {
	loader, err := core_config.NewLoader(core_config.MainConfig)
	if err != nil {
		panic(err)
	}

	cfg, err := loader.Load()
	if err != nil {
		panic(err)
	}

	log := logger.New(
		logger.WithComponent("app"),
		logger.WithLevelString(cfg.LoggerLevel),
	)

	db, err := postgres.Open(cfg.Database.DSN())
	if err != nil {
		panic(err)
	}
	defer db.Close()

	healthRepository := health_repository.NewStaticRepository("getytstatsapi")
	healthService := health_service.New(healthRepository)
	healthHandler := health_http.NewHandler(log.Named("health.http"), healthService)

	apiV1 := core_http_server.NewAPIVersionRouter(core_http_server.ApiVersion1)
	apiV1.RegisterRoutes(
		core_http_server.NewRoute(http.MethodGet, "/health", healthHandler.Get),
	)

	if strings.TrimSpace(cfg.YouTubeAPIKey.String()) == "" {
		log.Warn("stats routes are disabled: youtube api key is empty")
	} else {
		statsRepository, err := youtube_stats_repository.New(context.Background(), cfg.YouTubeAPIKey.String())
		if err != nil {
			panic(err)
		}

		historyStore := postgres_stats_repository.NewHistoryStore(db)
		recordingRepository := postgres_stats_repository.NewRecorder(statsRepository, historyStore)
		sponsorBlockRepository := sponsorblock_repository.New("")
		statsService := stats_service.New(recordingRepository, sponsorBlockRepository)
		statsHandler := stats_http.NewHandler(log.Named("stats.http"), statsService)

		apiV1.RegisterRoutes(
			core_http_server.NewRoute(http.MethodGet, "/stats/get", statsHandler.GetStats),
		)
	}

	server := core_http_server.NewHTTPServer(
		cfg.HTTP.Address,
		log.Named("http"),
		core_http_middleware.RequestID(),
		core_http_middleware.Logger(log.Named("http.middleware")),
		core_http_middleware.Trace(),
		core_http_middleware.Panic(),
	)
	server.RegisterAPIRouters(apiV1)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := server.Run(ctx); err != nil {
		panic(err)
	}
}
