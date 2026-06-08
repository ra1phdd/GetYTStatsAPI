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
	sponsorblock_service "getytstatsapi/internal/entities/sponsorblock/service"
	sponsorblock_http "getytstatsapi/internal/entities/sponsorblock/transport/http"
	postgres_stats_repository "getytstatsapi/internal/entities/stats/repository/postgres"
	youtube_stats_repository "getytstatsapi/internal/entities/stats/repository/youtube"
	stats_service "getytstatsapi/internal/entities/stats/service"
	stats_http "getytstatsapi/internal/entities/stats/transport/http"
	"getytstatsapi/pkg/logger"
	"net/http"
	"os/signal"
	"strings"
	"syscall"
)

func main() {
	store, err := core_config.LoadStore()
	if err != nil {
		panic(err)
	}

	var cfg *core_config.Config
	if err := store.Read(func(readCfg *core_config.Config) error {
		cfg = readCfg
		return nil
	}); err != nil {
		panic(err)
	}

	log := logger.New(
		logger.WithComponent("app"),
		logger.WithLevelString(cfg.LoggerLevel),
		logger.WithFile(core_config.ResolvePath("logs/app.log")),
	)

	db, err := postgres.Open(cfg.Database.DSN())
	if err != nil {
		panic(err)
	}
	defer db.Close()

	healthRepository := health_repository.NewStaticRepository("getytstatsapi")
	healthService := health_service.New(healthRepository)
	healthHandler := health_http.NewHandler(log.Named("health.http"), healthService)
	sponsorBlockRepository := sponsorblock_repository.New()
	sponsorBlockService := sponsorblock_service.New(sponsorBlockRepository)
	sponsorBlockHandler := sponsorblock_http.NewHandler(log.Named("sponsorblock.http"), sponsorBlockService)

	apiV1 := core_http_server.NewAPIVersionRouter(core_http_server.ApiVersion1)
	apiV1.RegisterRoutes(
		core_http_server.NewRoute(http.MethodGet, "/health", healthHandler.Get),
		core_http_server.NewRoute(http.MethodGet, "/sponsorblock/get", sponsorBlockHandler.GetSegments),
	)

	if strings.TrimSpace(cfg.Features.StintInside.YouTubeAPIKey.String()) == "" {
		log.Warn("stats routes are disabled: youtube api key is empty")
	} else {
		statsRepository, err := youtube_stats_repository.New(context.Background(), cfg.Features.StintInside.YouTubeAPIKey.String())
		if err != nil {
			panic(err)
		}

		historyStore := postgres_stats_repository.NewHistoryStore(db)
		recordingRepository := postgres_stats_repository.NewRecorder(statsRepository, historyStore)
		statsService := stats_service.New(recordingRepository)
		statsHandler := stats_http.NewFromConfig(cfg, log.Named("stats.http"), statsService)

		apiV1.RegisterRoutes(
			core_http_server.NewRoute(http.MethodGet, "/command/get", statsHandler.GetCommand),
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
