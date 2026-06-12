package main

import (
	"context"
	core_config "getytstatsapi/internal/core/infra/config"
	"getytstatsapi/internal/core/infra/postgres"
	core_http_middleware "getytstatsapi/internal/core/transport/http/middleware"
	core_http_server "getytstatsapi/internal/core/transport/http/server"
	campaign_bot_client "getytstatsapi/internal/entities/campaign/client/telegrambot"
	campaign_google "getytstatsapi/internal/entities/campaign/repository/google"
	campaign_postgres "getytstatsapi/internal/entities/campaign/repository/postgres"
	campaign_service "getytstatsapi/internal/entities/campaign/service"
	campaign_http "getytstatsapi/internal/entities/campaign/transport/http"
	health_repository "getytstatsapi/internal/entities/health/repository"
	health_service "getytstatsapi/internal/entities/health/service"
	health_http "getytstatsapi/internal/entities/health/transport/http"
	sponsorblock_repository "getytstatsapi/internal/entities/sponsorblock/repository/sponsorblock"
	postgres_stats_repository "getytstatsapi/internal/entities/stats/repository/postgres"
	youtube_stats_repository "getytstatsapi/internal/entities/stats/repository/youtube"
	stats_service "getytstatsapi/internal/entities/stats/service"
	stats_http "getytstatsapi/internal/entities/stats/transport/http"
	userauth_service "getytstatsapi/internal/entities/userauth/service"
	"net/http"
	"os/signal"
	"strings"
	"syscall"
	"time"

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
		panic("youtube api key is required for campaign api")
	}

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

	campaignStore := campaign_postgres.NewStore(db)
	googleRedirectURL := cfg.GoogleOAuth.RedirectURL
	if strings.TrimSpace(googleRedirectURL) == "" {
		googleRedirectURL = strings.TrimRight(strings.TrimSpace(cfg.PublicBaseURL), "/") + "/v1/google/callback"
	}
	googleClient := campaign_google.New(cfg.GoogleOAuth.ClientID, cfg.GoogleOAuth.ClientSecret.String(), googleRedirectURL)
	campaigns := campaign_service.New(campaignStore, statsService, statsRepository, googleClient, cfg.ExportJWTSecret.String(), cfg.PublicBaseURL, cfg.AccessJWTSecret.String())
	authService := userauth_service.New(campaignStore, cfg.TelegramAuth.BotToken.String(), cfg.AccessJWTSecret.String(), cfg.RefreshJWTSecret.String())
	campaignHandler := campaign_http.NewHandler(
		log.Named("campaign.http"),
		campaigns,
		authService,
		cfg.PublicBaseURL,
		cfg.Internal.PeerServiceID,
		cfg.Internal.PeerServiceSecret.String(),
	)

	apiV1.RegisterRoutes(
		core_http_server.NewRoute(http.MethodPost, "/auth/telegram", campaignHandler.AuthTelegram),
		core_http_server.NewRoute(http.MethodPost, "/auth/refresh", campaignHandler.AuthRefresh),
		core_http_server.NewRoute(http.MethodPost, "/auth/logout", campaignHandler.AuthLogout),
		core_http_server.NewRoute(http.MethodGet, "/google/callback", campaignHandler.CompleteGoogleLink),
		core_http_server.NewRoute(http.MethodGet, "/me", campaignHandler.Me),
		core_http_server.NewRoute(http.MethodGet, "/users/{user_id}/channels", campaignHandler.GetUserChannels),
		core_http_server.NewRoute(http.MethodPost, "/users/{user_id}/channels/resolve", campaignHandler.ResolveUserChannel),
		core_http_server.NewRoute(http.MethodPost, "/users/{user_id}/channels/verify", campaignHandler.VerifyUserChannel),
		core_http_server.NewRoute(http.MethodPost, "/users/{user_id}/channels", campaignHandler.CreateUserChannel),
		core_http_server.NewRoute(http.MethodDelete, "/users/{user_id}/channels/{channel_id}", campaignHandler.DeleteUserChannel),
		core_http_server.NewRoute(http.MethodPost, "/users/{user_id}/google/link", campaignHandler.GetGoogleLink),
		core_http_server.NewRoute(http.MethodGet, "/users/{user_id}/campaigns", campaignHandler.GetUserCampaigns),
		core_http_server.NewRoute(http.MethodPost, "/users/{user_id}/campaigns", campaignHandler.CreateUserCampaign),
		core_http_server.NewRoute(http.MethodGet, "/users/{user_id}/campaigns/{campaign_id}", campaignHandler.GetUserCampaign),
		core_http_server.NewRoute(http.MethodPost, "/users/{user_id}/campaigns/{campaign_id}/close", campaignHandler.CloseUserCampaign),
		core_http_server.NewRoute(http.MethodPost, "/users/{user_id}/campaigns/{campaign_id}/refresh", campaignHandler.RefreshUserCampaign),
		core_http_server.NewRoute(http.MethodPost, "/users/{user_id}/campaigns/{campaign_id}/spreadsheet", campaignHandler.CreateCampaignSpreadsheet),
		core_http_server.NewRoute(http.MethodGet, "/users/{user_id}/settings", campaignHandler.GetUserSettings),
		core_http_server.NewRoute(http.MethodPatch, "/users/{user_id}/settings", campaignHandler.PatchUserSettings),
		core_http_server.NewRoute(http.MethodGet, "/users/{user_id}/input-session", campaignHandler.GetUserInputSession),
		core_http_server.NewRoute(http.MethodPut, "/users/{user_id}/input-session", campaignHandler.PutUserInputSession),
		core_http_server.NewRoute(http.MethodDelete, "/users/{user_id}/input-session", campaignHandler.DeleteUserInputSession),
		core_http_server.NewRoute(http.MethodGet, "/campaigns/export/{token}", campaignHandler.ExportCampaign),
	)

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

	botNotifier := campaign_bot_client.New(
		cfg.Notifications.WebhookURL,
		cfg.Internal.ServiceID,
		cfg.Internal.ServiceSecret.String(),
	)
	go runNotificationLoop(ctx, log.Named("campaign.notifications"), campaigns, botNotifier)

	if err := server.Run(ctx); err != nil {
		panic(err)
	}
}

func runNotificationLoop(ctx context.Context, log *logger.Logger, campaigns *campaign_service.Service, notifier *campaign_bot_client.Client) {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := campaigns.ProcessDueNotifications(ctx, notifier); err != nil {
				log.Error("failed to process due notifications", logger.Err(err))
			}
		}
	}
}
